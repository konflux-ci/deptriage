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
	"errors"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v85/github"
)

// PRData holds the metadata fetched from a pull request.
type PRData struct {
	Number  int
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
