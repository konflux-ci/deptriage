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
	"net/http"
	"strings"
	"time"

	gh "github.com/google/go-github/v85/github"
)

// ErrMergeQueueRequired indicates the repository requires PRs to go through a merge queue.
var ErrMergeQueueRequired = errors.New("merge queue required")

// PRData holds the metadata fetched from a pull request.
type PRData struct {
	Number  int
	NodeID  string
	Title   string
	Body    string
	Author  string
	BaseRef string
	HeadRef string
	Labels  []string
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

	return &PRData{
		Number:  number,
		NodeID:  pr.GetNodeID(),
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		Author:  pr.GetUser().GetLogin(),
		BaseRef: pr.GetBase().GetRef(),
		HeadRef: pr.GetHead().GetRef(),
		Labels:  labels,
	}, nil
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
	_, _, err := c.inner.PullRequests.Merge(ctx, c.owner, c.repo, prNumber, "", &gh.PullRequestOptions{
		MergeMethod: method,
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

// ChecksAllPassed evaluates CI checks on the PR head SHA, excluding the specified
// workflow name (to avoid self-referencing). Returns ChecksPassed, ChecksInProgress,
// or ChecksFailed.
func (c *Client) ChecksAllPassed(ctx context.Context, prNumber int, excludeWorkflow string) (CheckStatus, error) {
	pr, _, err := c.inner.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return ChecksFailed, fmt.Errorf("fetching PR #%d for check status: %w", prNumber, err)
	}
	ref := pr.GetHead().GetSHA()

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
			if conclusion != "success" && conclusion != "neutral" && conclusion != "skipped" && conclusion != "cancelled" {
				return ChecksFailed, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Also check combined commit status (legacy status API)
	status, _, err := c.inner.Repositories.GetCombinedStatus(ctx, c.owner, c.repo, ref, nil)
	if err != nil {
		return ChecksFailed, fmt.Errorf("fetching combined status for %s: %w", ref, err)
	}
	for _, s := range status.Statuses {
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

	if hasInProgress {
		return ChecksInProgress, nil
	}
	return ChecksPassed, nil
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
