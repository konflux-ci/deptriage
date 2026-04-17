/*
Copyright 2025 Red Hat, Inc.

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
	"strings"

	gh "github.com/google/go-github/v84/github"
)

const (
	analysisMarker  = "<!-- deptriage-analysis -->"
	maxCommentBytes = 65000 // GitHub's limit is 65536, leave margin

	AnalysisHeader = "## AI Dependency Impact Analysis"
)

// UpsertAnalysisComment finds an existing analysis comment by marker, collapses
// its content into a <details> block, and appends the new analysis. Creates a
// new comment if none exists.
func (c *Client) UpsertAnalysisComment(ctx context.Context, prNumber int, body string) error {
	existing, err := c.findAnalysisComment(ctx, prNumber)
	if err != nil {
		return err
	}

	fullBody := fmt.Sprintf("%s\n%s\n\n%s", analysisMarker, AnalysisHeader, body)

	if existing != nil {
		// Collapse previous content into <details>
		previousContent := stripMarkerAndHeader(existing.GetBody())
		collapsed := fmt.Sprintf("<details><summary>Previous analysis</summary>\n\n%s\n</details>\n\n", previousContent)
		fullBody = fmt.Sprintf("%s\n%s\n\n%s%s", analysisMarker, AnalysisHeader, collapsed, body)

		// Truncate if needed
		fullBody = truncateComment(fullBody)

		_, _, err = c.inner.Issues.EditComment(ctx, c.owner, c.repo, existing.GetID(), &gh.IssueComment{
			Body: gh.Ptr(fullBody),
		})
		if err != nil {
			return fmt.Errorf("updating analysis comment: %w", err)
		}
		return nil
	}

	fullBody = truncateComment(fullBody)
	_, _, err = c.inner.Issues.CreateComment(ctx, c.owner, c.repo, prNumber, &gh.IssueComment{
		Body: gh.Ptr(fullBody),
	})
	if err != nil {
		return fmt.Errorf("creating analysis comment: %w", err)
	}
	return nil
}

// PostFallbackComment posts a fallback comment when analysis is unavailable.
func (c *Client) PostFallbackComment(ctx context.Context, prNumber int, reason string) error {
	body := fmt.Sprintf("> **Analysis unavailable**: %s. This does not block the PR — please review the dependency update manually.", reason)
	return c.UpsertAnalysisComment(ctx, prNumber, body)
}

// SubmitReview submits a formal PR review (APPROVE, REQUEST_CHANGES, or COMMENT).
func (c *Client) SubmitReview(ctx context.Context, prNumber int, event, body string) error {
	_, _, err := c.inner.PullRequests.CreateReview(ctx, c.owner, c.repo, prNumber, &gh.PullRequestReviewRequest{
		Body:  gh.Ptr(body),
		Event: gh.Ptr(event),
	})
	if err != nil {
		return fmt.Errorf("submitting %s review on PR #%d: %w", event, prNumber, err)
	}
	return nil
}

func (c *Client) findAnalysisComment(ctx context.Context, prNumber int) (*gh.IssueComment, error) {
	opts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		comments, resp, err := c.inner.Issues.ListComments(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("listing PR comments: %w", err)
		}
		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), analysisMarker) {
				return comment, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return nil, nil
}

func stripMarkerAndHeader(body string) string {
	body = strings.Replace(body, analysisMarker, "", 1)
	body = strings.Replace(body, AnalysisHeader, "", 1)
	return strings.TrimSpace(body)
}

func truncateComment(body string) string {
	if len(body) <= maxCommentBytes {
		return body
	}
	// Find and remove oldest <details> blocks until we fit
	for len(body) > maxCommentBytes {
		start := strings.Index(body, "<details>")
		end := strings.Index(body, "</details>")
		if start == -1 || end == -1 {
			break
		}
		body = body[:start] + body[end+len("</details>"):]
	}
	if len(body) > maxCommentBytes {
		body = body[:maxCommentBytes] + "\n\n> *[Truncated due to comment size limit]*"
	}
	return body
}
