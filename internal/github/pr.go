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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gh "github.com/google/go-github/v86/github"
)

// ErrMergeQueueRequired indicates the repository requires PRs to go through a merge queue.
var ErrMergeQueueRequired = errors.New("merge queue required")

// PRData holds the metadata fetched from a pull request.
type PRData struct {
	Number    int
	NodeID    string
	Title     string
	Body      string
	Author    string
	BaseRef   string
	BaseSHA   string
	HeadRef   string
	HeadSHA   string
	HeadOwner string
	HeadRepo  string
	Labels    []string
}

// FetchPR retrieves pull request metadata.
func (c *Client) FetchPR(ctx context.Context, number int) (*PRData, error) {
	pr, _, err := c.inner.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("fetching PR #%d: %w", number, err)
	}

	var labels []string
	for _, l := range pr.Labels {
		labels = append(labels, l.GetName())
	}

	head := pr.GetHead()
	headRepo := head.GetRepo()
	return &PRData{
		Number:    number,
		NodeID:    pr.GetNodeID(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		Author:    pr.GetUser().GetLogin(),
		BaseRef:   pr.GetBase().GetRef(),
		BaseSHA:   pr.GetBase().GetSHA(),
		HeadRef:   head.GetRef(),
		HeadSHA:   head.GetSHA(),
		HeadOwner: headRepo.GetOwner().GetLogin(),
		HeadRepo:  headRepo.GetName(),
		Labels:    labels,
	}, nil
}

// FetchPRSnapshot returns commit provenance and changed paths for the exact
// base-to-head commit range captured in pr. Unlike pull-request endpoints,
// the compare endpoint is addressed by immutable commit SHAs.
func (c *Client) FetchPRSnapshot(ctx context.Context, pr *PRData) ([]CommitInfo, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if pr == nil || pr.BaseSHA == "" || pr.HeadSHA == "" || pr.HeadOwner == "" || pr.HeadRepo == "" {
		return nil, nil, fmt.Errorf("fetching PR snapshot: base SHA, head SHA, and head repository are required")
	}

	opts := &gh.ListOptions{PerPage: 100}
	var commits []CommitInfo
	var files []string
	for {
		comparison, response, err := c.inner.Repositories.CompareCommits(ctx, pr.HeadOwner, pr.HeadRepo, pr.BaseSHA, pr.HeadSHA, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("comparing %s to %s in %s/%s: %w", pr.BaseSHA, pr.HeadSHA, pr.HeadOwner, pr.HeadRepo, err)
		}
		for _, commit := range comparison.Commits {
			author := ""
			if commit.GetAuthor() != nil {
				author = commit.GetAuthor().GetLogin()
			}
			commits = append(commits, CommitInfo{SHA: commit.GetSHA(), Author: author})
		}
		// GitHub includes files only on the first comparison page.
		if opts.Page == 0 || opts.Page == 1 {
			for _, file := range comparison.Files {
				files = append(files, file.GetFilename())
			}
		}
		if response.NextPage == 0 {
			if comparison.GetTotalCommits() != len(commits) {
				return nil, nil, fmt.Errorf("comparing %s to %s: received %d of %d commits", pr.BaseSHA, pr.HeadSHA, len(commits), comparison.GetTotalCommits())
			}
			break
		}
		opts.Page = response.NextPage
	}
	// The compare API returns at most 300 files and does not indicate whether
	// there are more. Fail closed at that boundary rather than approving a
	// range whose full file scope cannot be verified.
	if len(files) >= 300 {
		return nil, nil, fmt.Errorf("comparing %s to %s: changed-file list may be truncated", pr.BaseSHA, pr.HeadSHA)
	}
	return commits, files, nil
}

// EnsureLabel creates the label if it doesn't exist, then applies it to the PR.
func (c *Client) EnsureLabel(ctx context.Context, prNumber int, name, color, description string) error {
	// Create label if needed (ignore "already_exists" error)
	_, _, err := c.inner.Issues.CreateLabel(ctx, c.owner, c.repo, &gh.Label{
		Name:        gh.Ptr(name),
		Color:       gh.Ptr(color),
		Description: gh.Ptr(description),
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("creating label %q: %w", name, err)
	}

	_, _, err = c.inner.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, prNumber, []string{name})
	if err != nil {
		return fmt.Errorf("applying label %q to PR #%d: %w", name, prNumber, err)
	}
	return nil
}

// RemoveLabel removes a label from a PR. Ignores "not found" errors.
func (c *Client) RemoveLabel(ctx context.Context, prNumber int, name string) error {
	_, err := c.inner.Issues.RemoveLabelForIssue(ctx, c.owner, c.repo, prNumber, name)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("removing label %q from PR #%d: %w", name, prNumber, err)
	}
	return nil
}

// isAlreadyExists checks for GitHub's "already_exists" error code, which has no
// built-in helper in go-github — the library requires inspecting ErrorResponse.Errors.
func isAlreadyExists(err error) bool {
	var ghErr *gh.ErrorResponse
	if errors.As(err, &ghErr) {
		for _, e := range ghErr.Errors {
			if e.Code == "already_exists" {
				return true
			}
		}
	}
	return false
}

func isNotFound(err error) bool {
	var ghErr *gh.ErrorResponse
	return errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound
}

// FindOpenPRsForSHA returns PR numbers for open PRs whose head SHA matches.
func (c *Client) FindOpenPRsForSHA(ctx context.Context, sha string) ([]int, error) {
	opts := &gh.PullRequestListOptions{
		State:       "open",
		ListOptions: gh.ListOptions{PerPage: 50},
	}
	var result []int
	for {
		prs, resp, err := c.inner.PullRequests.List(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("listing PRs: %w", err)
		}
		for _, pr := range prs {
			if pr.GetHead().GetSHA() == sha {
				result = append(result, pr.GetNumber())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return result, nil
}

// MergePR merges a pull request using the specified method (merge, squash, rebase).
// Returns ErrMergeQueueRequired if the repository requires PRs to go through a merge queue.
func (c *Client) MergePR(ctx context.Context, prNumber int, method string) error {
	return c.MergePRAtSHA(ctx, prNumber, method, "")
}

// MergePRAtSHA merges a pull request only if its head still matches sha. An
// empty sha preserves the GitHub API's default merge behavior.
func (c *Client) MergePRAtSHA(ctx context.Context, prNumber int, method, sha string) error {
	_, _, err := c.inner.PullRequests.Merge(ctx, c.owner, c.repo, prNumber, "", &gh.PullRequestOptions{
		MergeMethod: method,
		SHA:         sha,
	})
	if err != nil {
		if isMergeQueueError(err) {
			return fmt.Errorf("merging PR #%d: %w", prNumber, ErrMergeQueueRequired)
		}
		return fmt.Errorf("merging PR #%d: %w", prNumber, err)
	}
	return nil
}

func isMergeQueueError(err error) bool {
	var ghErr *gh.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusMethodNotAllowed {
		return strings.Contains(ghErr.Message, "merge queue") ||
			strings.Contains(strings.ToLower(ghErr.Message), "queue")
	}
	return false
}

// EnqueuePR adds a pull request to the repository's merge queue via the GraphQL API.
func (c *Client) EnqueuePR(ctx context.Context, prNodeID string) error {
	query := `mutation($prID: ID!) {
		enqueuePullRequest(input: {pullRequestId: $prID}) {
			mergeQueueEntry {
				id
			}
		}
	}`

	payload := map[string]any{
		"query":     query,
		"variables": map[string]string{"prID": prNodeID},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating GraphQL request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing GraphQL request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading GraphQL response: %w", err)
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parsing GraphQL response: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("enqueuing PR: %s", result.Errors[0].Message)
	}

	return nil
}

// CheckStatus represents the result of evaluating all CI checks on a PR.
type CheckStatus int

const (
	ChecksPassed     CheckStatus = iota // All checks completed successfully
	ChecksInProgress                    // Some checks are still running
	ChecksFailed                        // At least one check has failed
)

// ChecksAllPassed evaluates CI check runs and legacy commit statuses on the PR head SHA,
// excluding the specified workflow name (to avoid self-referencing). Returns ChecksPassed,
// ChecksInProgress, or ChecksFailed. If the token lacks permission to read legacy commit
// statuses (403), the legacy check is skipped gracefully.
func (c *Client) ChecksAllPassed(ctx context.Context, prNumber int, excludeWorkflow string) (CheckStatus, error) {
	pr, _, err := c.inner.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return ChecksFailed, fmt.Errorf("fetching PR #%d for check status: %w", prNumber, err)
	}
	return c.ChecksAllPassedForSHA(ctx, pr.GetHead().GetSHA(), excludeWorkflow)
}

// ChecksAllPassedForSHA evaluates CI checks for an explicit commit SHA. Callers
// handling events should use this method to avoid changing the checked commit
// between event processing and merge.
func (c *Client) ChecksAllPassedForSHA(ctx context.Context, ref, excludeWorkflow string) (CheckStatus, error) {

	hasInProgress := false

	opts := &gh.ListCheckRunsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		result, resp, err := c.inner.Checks.ListCheckRunsForRef(ctx, c.owner, c.repo, ref, opts)
		if err != nil {
			return ChecksFailed, fmt.Errorf("listing check runs for %s: %w", ref, err)
		}
		for _, cr := range result.CheckRuns {
			if cr.GetName() == excludeWorkflow {
				continue
			}
			status := cr.GetStatus()
			if status != "completed" {
				hasInProgress = true
				continue
			}
			conclusion := cr.GetConclusion()
			if conclusion != "success" && conclusion != "neutral" && conclusion != "skipped" {
				return ChecksFailed, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Also check combined commit status (legacy status API).
	// If the token lacks statuses:read permission (403), skip gracefully.
	combinedStatus, resp, err := c.inner.Repositories.GetCombinedStatus(ctx, c.owner, c.repo, ref, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			slog.Warn("commit status API not accessible, skipping legacy status check", "ref", ref)
		} else {
			return ChecksFailed, fmt.Errorf("fetching combined status for %s: %w", ref, err)
		}
	} else {
		for _, s := range combinedStatus.Statuses {
			if s.GetContext() == excludeWorkflow {
				continue
			}
			state := s.GetState()
			if state == "pending" {
				hasInProgress = true
				continue
			}
			if state != "success" {
				return ChecksFailed, nil
			}
		}
	}

	if hasInProgress {
		return ChecksInProgress, nil
	}
	return ChecksPassed, nil
}

// CommitInfo holds the author login and SHA of a single commit.
type CommitInfo struct {
	SHA    string
	Author string
}

// FetchPRCommits returns the author login and SHA for every commit on a PR.
func (c *Client) FetchPRCommits(ctx context.Context, prNumber int) ([]CommitInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	opts := &gh.ListOptions{PerPage: 100}
	var result []CommitInfo
	for {
		commits, resp, err := c.inner.PullRequests.ListCommits(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing commits for PR #%d: %w", prNumber, err)
		}
		for _, commit := range commits {
			author := ""
			if commit.GetAuthor() != nil {
				author = commit.GetAuthor().GetLogin()
			}
			result = append(result, CommitInfo{
				SHA:    commit.GetSHA(),
				Author: author,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return result, nil
}

// FetchPRFiles returns the file paths changed in a PR.
func (c *Client) FetchPRFiles(ctx context.Context, prNumber int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	opts := &gh.ListOptions{PerPage: 100}
	var result []string
	for {
		files, resp, err := c.inner.PullRequests.ListFiles(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing files for PR #%d: %w", prNumber, err)
		}
		for _, f := range files {
			result = append(result, f.GetFilename())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return result, nil
}

// FetchSubmodulePaths returns the paths of git submodules in the repo tree at
// the given ref by looking for tree entries with mode "160000" (gitlink).
func (c *Client) FetchSubmodulePaths(ctx context.Context, ref string) ([]string, error) {
	return c.FetchSubmodulePathsForRepo(ctx, c.owner, c.repo, ref)
}

// FetchSubmodulePathsForRepo returns every submodule path in the repository
// tree at ref. The recursive tree query is required to detect nested gitlinks.
func (c *Client) FetchSubmodulePathsForRepo(ctx context.Context, owner, repo, ref string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if owner == "" || repo == "" || ref == "" {
		return nil, fmt.Errorf("fetching tree: owner, repository, and ref are required")
	}
	tree, _, err := c.inner.Git.GetTree(ctx, owner, repo, ref, true)
	if err != nil {
		return nil, fmt.Errorf("fetching tree for %s/%s at %s: %w", owner, repo, ref, err)
	}
	if tree.GetTruncated() {
		return nil, fmt.Errorf("fetching tree for %s/%s at %s: recursive tree response truncated", owner, repo, ref)
	}
	var paths []string
	for _, entry := range tree.Entries {
		if entry.GetMode() == "160000" {
			paths = append(paths, entry.GetPath())
		}
	}
	return paths, nil
}

// HasLabels returns true if the PR has all the specified labels.
func (c *Client) HasLabels(ctx context.Context, prNumber int, labels []string) (bool, error) {
	pr, err := c.FetchPR(ctx, prNumber)
	if err != nil {
		return false, err
	}
	labelSet := make(map[string]bool, len(pr.Labels))
	for _, l := range pr.Labels {
		labelSet[l] = true
	}
	for _, required := range labels {
		if !labelSet[required] {
			return false, nil
		}
	}
	return true, nil
}
