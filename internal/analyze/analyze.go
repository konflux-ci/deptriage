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

package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/konflux-ci/deptriage/internal/types"

	gh "github.com/google/go-github/v86/github"
	"golang.org/x/oauth2"
)

// Options configures deterministic dependency inspection.
type Options struct {
	Token          string
	ClassifyOutput string
	ContextOutput  string
	WorkDir        string
}

// Run gathers import, advisory, and vulnerability evidence and writes it to a
// local JSON file. It performs no model request and no GitHub write action.
func Run(ctx context.Context, opts Options) error {
	if err := removeStaleReport(opts.ContextOutput); err != nil {
		return err
	}
	classifyResult, err := readClassifyOutput(opts.ClassifyOutput)
	if err != nil {
		return err
	}

	var rawGHClient *gh.Client
	if opts.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
		rawGHClient = gh.NewClient(oauth2.NewClient(ctx, ts))
	}
	report := GatherContext(ctx, classifyResult, rawGHClient, opts.WorkDir)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing deterministic inspection report: %w", err)
	}
	if err := os.WriteFile(opts.ContextOutput, data, 0644); err != nil {
		return fmt.Errorf("writing deterministic inspection report %s: %w", opts.ContextOutput, err)
	}
	return nil
}

func removeStaleReport(path string) error {
	if path == "" {
		return fmt.Errorf("deterministic inspection report path is required")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale deterministic inspection report %s: %w", path, err)
	}
	return nil
}

func readClassifyOutput(path string) (*types.ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var result types.ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &result, nil
}
