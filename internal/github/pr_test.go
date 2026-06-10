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
