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

	gh "github.com/google/go-github/v86/github"
)

// SubmitReview submits a formal PR review (APPROVE, REQUEST_CHANGES, or COMMENT).
func (c *Client) SubmitReview(ctx context.Context, prNumber int, event, body string) error {
	return c.SubmitReviewAtSHA(ctx, prNumber, event, body, "")
}

// SubmitReviewAtSHA submits a formal PR review for the specified commit. An
// empty SHA preserves GitHub's default behavior of reviewing the current head.
func (c *Client) SubmitReviewAtSHA(ctx context.Context, prNumber int, event, body, sha string) error {
	review := &gh.PullRequestReviewRequest{Body: gh.Ptr(body), Event: gh.Ptr(event)}
	if sha != "" {
		review.CommitID = gh.Ptr(sha)
	}
	_, _, err := c.inner.PullRequests.CreateReview(ctx, c.owner, c.repo, prNumber, review)
	if err != nil {
		return fmt.Errorf("submitting %s review on PR #%d: %w", event, prNumber, err)
	}
	return nil
}
