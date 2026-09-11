/*
Copyright 2026 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v86/github"
)

// newTestClient creates a Client backed by the given httptest.Server.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	ghClient := gh.NewClient(server.Client())
	u, _ := url.Parse(server.URL + "/")
	ghClient.BaseURL = u
	return &Client{
		inner: ghClient,
		owner: "testorg",
		repo:  "testrepo",
	}
}

func TestMergePRAtSHA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /repos/testorg/testrepo/pulls/1/merge", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var request struct {
			SHA         string `json:"sha"`
			MergeMethod string `json:"merge_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decoding merge request: %v", err)
		}
		if request.SHA != "expected-sha" {
			t.Errorf("merge SHA = %q, want expected-sha", request.SHA)
		}
		if request.MergeMethod != "squash" {
			t.Errorf("merge method = %q, want squash", request.MergeMethod)
		}
		_, _ = fmt.Fprint(w, `{"merged": true}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	if err := newTestClient(t, server).MergePRAtSHA(context.Background(), 1, "squash", "expected-sha"); err != nil {
		t.Fatalf("MergePRAtSHA() error = %v", err)
	}
}

func TestSubmitReviewAtSHA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/testorg/testrepo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var request struct {
			CommitID string `json:"commit_id"`
			Event    string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decoding review request: %v", err)
		}
		if request.CommitID != "expected-sha" {
			t.Errorf("review commit ID = %q, want expected-sha", request.CommitID)
		}
		if request.Event != "APPROVE" {
			t.Errorf("review event = %q, want APPROVE", request.Event)
		}
		_, _ = fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	if err := newTestClient(t, server).SubmitReviewAtSHA(context.Background(), 1, "APPROVE", "approved", "expected-sha"); err != nil {
		t.Fatalf("SubmitReviewAtSHA() error = %v", err)
	}
}

func TestSubmitReviewOmitsEmptyCommitID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/testorg/testrepo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decoding review request: %v", err)
		}
		if _, ok := request["commit_id"]; ok {
			t.Errorf("legacy SubmitReview request unexpectedly included commit_id: %#v", request["commit_id"])
		}
		_, _ = fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	if err := newTestClient(t, server).SubmitReview(context.Background(), 1, "COMMENT", "comment"); err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}
}

func TestChecksAllPassedForSHACancelledCheckFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/commits/test-sha/check-runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"total_count": 1, "check_runs": [{"name": "build", "status": "completed", "conclusion": "cancelled"}]}`)
	})
	mux.HandleFunc("GET /repos/testorg/testrepo/commits/test-sha/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"state": "success", "statuses": []}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	status, err := newTestClient(t, server).ChecksAllPassedForSHA(context.Background(), "test-sha", "")
	if err != nil {
		t.Fatalf("ChecksAllPassedForSHA() error = %v", err)
	}
	if status != ChecksFailed {
		t.Errorf("ChecksAllPassedForSHA() = %v, want ChecksFailed for cancelled check", status)
	}
}

func TestFetchPRRecordsHeadRepository(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
            "number": 1,
			"base": {"ref": "main", "sha": "base-sha"},
            "head": {
                "ref": "dependency-update",
                "sha": "head-sha",
                "repo": {"name": "fork-repo", "owner": {"login": "fork-owner"}}
            }
        }`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	pr, err := newTestClient(t, server).FetchPR(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.HeadOwner != "fork-owner" || pr.HeadRepo != "fork-repo" || pr.HeadSHA != "head-sha" || pr.BaseSHA != "base-sha" {
		t.Errorf("snapshot metadata = base %s, head %s/%s at %s; want base-sha and fork-owner/fork-repo at head-sha", pr.BaseSHA, pr.HeadOwner, pr.HeadRepo, pr.HeadSHA)
	}
}

func TestFetchPRSnapshotUsesCapturedSHAs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/fork-owner/fork-repo/compare/base-sha...head-sha", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want 100", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
          "total_commits": 1,
          "commits": [{"sha": "head-sha", "author": {"login": "renovate[bot]"}}],
          "files": [{"filename": "go.mod"}, {"filename": "go.sum"}]
        }`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	commits, files, err := newTestClient(t, server).FetchPRSnapshot(context.Background(), &PRData{
		BaseSHA: "base-sha", HeadSHA: "head-sha", HeadOwner: "fork-owner", HeadRepo: "fork-repo",
	})
	if err != nil {
		t.Fatalf("FetchPRSnapshot() error = %v", err)
	}
	if len(commits) != 1 || commits[0] != (CommitInfo{SHA: "head-sha", Author: "renovate[bot]"}) {
		t.Errorf("commits = %+v, want trusted head commit", commits)
	}
	if len(files) != 2 || files[0] != "go.mod" || files[1] != "go.sum" {
		t.Errorf("files = %v, want [go.mod go.sum]", files)
	}
}

// --- FetchPRCommits tests ---

func TestFetchPRCommits_SinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[
			{"sha": "aaa111", "author": {"login": "renovate[bot]"}},
			{"sha": "bbb222", "author": {"login": "renovate[bot]"}}
		]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	commits, err := client.FetchPRCommits(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].SHA != "aaa111" || commits[0].Author != "renovate[bot]" {
		t.Errorf("commit 0: got %+v", commits[0])
	}
	if commits[1].SHA != "bbb222" || commits[1].Author != "renovate[bot]" {
		t.Errorf("commit 1: got %+v", commits[1])
	}
}

func TestFetchPRCommits_Pagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=2>; rel="next"`, "http://"+r.Host, r.URL.Path))
			_, _ = fmt.Fprint(w, `[{"sha": "aaa", "author": {"login": "bot1"}}]`)
		default:
			_, _ = fmt.Fprint(w, `[{"sha": "bbb", "author": {"login": "bot1"}}]`)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server)
	commits, err := client.FetchPRCommits(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
}

func TestFetchPRCommits_NilAuthor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"sha": "ccc333", "author": null}]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	commits, err := client.FetchPRCommits(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
	if commits[0].Author != "" {
		t.Errorf("expected empty author for null, got %q", commits[0].Author)
	}
}

func TestFetchPRCommits_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/commits", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message": "internal error"}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.FetchPRCommits(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRCommits_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	commits, err := client.FetchPRCommits(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("got %d commits, want 0", len(commits))
	}
}

// --- FetchPRFiles tests ---

func TestFetchPRFiles_SinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[
			{"filename": "go.mod", "status": "modified"},
			{"filename": "go.sum", "status": "modified"}
		]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	files, err := client.FetchPRFiles(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0] != "go.mod" || files[1] != "go.sum" {
		t.Errorf("got files %v", files)
	}
}

func TestFetchPRFiles_Pagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=2>; rel="next"`, "http://"+r.Host, r.URL.Path))
			_, _ = fmt.Fprint(w, `[{"filename": "go.mod"}]`)
		default:
			_, _ = fmt.Fprint(w, `[{"filename": "go.sum"}]`)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server)
	files, err := client.FetchPRFiles(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}

func TestFetchPRFiles_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"message": "Resource not accessible"}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.FetchPRFiles(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRFiles_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	files, err := client.FetchPRFiles(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
}

// --- FetchSubmodulePaths tests ---

func TestFetchSubmodulePaths(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			t.Errorf("recursive query = %q, want 1", r.URL.Query().Get("recursive"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"sha": "abc123",
			"tree": [
				{"path": "go.mod", "mode": "100644", "type": "blob", "sha": "aaa"},
				{"path": "go.sum", "mode": "100644", "type": "blob", "sha": "bbb"},
				{"path": "internal", "mode": "040000", "type": "tree", "sha": "ccc"},
				{"path": "oauth2-proxy", "mode": "160000", "type": "commit", "sha": "ddd"},
				{"path": "another-submodule", "mode": "160000", "type": "commit", "sha": "eee"},
				{"path": "vendor/nested-submodule", "mode": "160000", "type": "commit", "sha": "fff"}
			]
		}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	paths, err := client.FetchSubmodulePaths(context.Background(), "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("got %d submodule paths, want 3: %v", len(paths), paths)
	}
	if paths[0] != "oauth2-proxy" || paths[1] != "another-submodule" || paths[2] != "vendor/nested-submodule" {
		t.Errorf("got paths %v, want nested submodule path", paths)
	}
}

func TestFetchSubmodulePathsForRepo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/fork-owner/fork-repo/git/trees/head-sha", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			t.Errorf("recursive query = %q, want 1", r.URL.Query().Get("recursive"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tree": [{"path": "vendor/nested", "mode": "160000", "type": "commit"}]}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	paths, err := newTestClient(t, server).FetchSubmodulePathsForRepo(context.Background(), "fork-owner", "fork-repo", "head-sha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "vendor/nested" {
		t.Errorf("got paths %v, want [vendor/nested]", paths)
	}
}

func TestFetchSubmodulePaths_NoSubmodules(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"sha": "abc123",
			"tree": [
				{"path": "go.mod", "mode": "100644", "type": "blob", "sha": "aaa"},
				{"path": "internal", "mode": "040000", "type": "tree", "sha": "bbb"}
			]
		}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	paths, err := client.FetchSubmodulePaths(context.Background(), "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("got %d submodule paths, want 0", len(paths))
	}
}

func TestFetchSubmodulePaths_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message": "Not Found"}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.FetchSubmodulePaths(context.Background(), "main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRFiles_ManyFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/testorg/testrepo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[
			{"filename": "go.mod"},
			{"filename": "go.sum"},
			{"filename": "vendor/github.com/foo/bar/baz.go"},
			{"filename": ".claude/settings.json"},
			{"filename": "internal/main.go"}
		]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	files, err := client.FetchPRFiles(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("got %d files, want 5", len(files))
	}
	if files[3] != ".claude/settings.json" {
		t.Errorf("expected .claude/settings.json at index 3, got %q", files[3])
	}
}
