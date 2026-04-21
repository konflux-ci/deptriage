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
	"strings"

	gh "github.com/google/go-github/v85/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client.
type Client struct {
	inner *gh.Client
	owner string
	repo  string
}

// NewClient creates an authenticated GitHub client for the given repository.
func NewClient(ctx context.Context, token, repoFullName string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := gh.NewClient(tc)

	owner, repo := splitRepo(repoFullName)
	return &Client{
		inner: client,
		owner: owner,
		repo:  repo,
	}
}

func splitRepo(fullName string) (string, string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return fullName, ""
	}
	return parts[0], parts[1]
}
