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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/konflux-ci/deptriage/internal/analyze"
	"github.com/konflux-ci/deptriage/internal/classify"
	"github.com/konflux-ci/deptriage/internal/merge"
)

const (
	flagProvider       = "provider"
	flagAPIKey         = "api-key"
	flagModel          = "model"
	flagAutoApprove    = "auto-approve"
	flagAutoMerge      = "auto-merge"
	flagClassifyOutput = "classify-output"
	flagDryRun         = "dry-run"
)

var (
	prNumber    int
	repo        string
	githubToken string
	outputFile  string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "deptriage",
	Short: "Dependency PR triage and AI impact analysis",
	Long:  "Classify dependency PRs by semver bump type, detect risk patterns, and run AI-assisted impact analysis.",
}

var classifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Classify a dependency PR by semver bump type and risk level",
	RunE:  runClassify,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Run AI-assisted impact analysis on a dependency PR",
	RunE:  runAnalyze,
}

var bothCmd = &cobra.Command{
	Use:   "both",
	Short: "Run classify then analyze in sequence",
	RunE:  runBoth,
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge eligible dependency PRs that have been approved and pass all checks",
	RunE:  runMerge,
}

func init() {
	// Shared flags with env var defaults for GitHub Actions
	for _, cmd := range []*cobra.Command{classifyCmd, analyzeCmd, bothCmd, mergeCmd} {
		cmd.Flags().IntVar(&prNumber, "pr-number", envInt("INPUT_PR_NUMBER", 0), "Pull request number")
		cmd.Flags().StringVar(&repo, "repo", envStr("GITHUB_REPOSITORY", ""), "Repository in owner/name format")
		cmd.Flags().StringVar(&githubToken, "github-token", envStr("INPUT_GITHUB_TOKEN", ""), "GitHub token for API operations")
		cmd.Flags().Bool(flagDryRun, envBool("INPUT_DRY_RUN"), "Suppress all GitHub API writes; log what would happen")
	}

	// Classify-specific flags
	classifyCmd.Flags().StringVar(&outputFile, "output", "/tmp/deptriage-classify.json", "Output file for classify result JSON")
	classifyCmd.Flags().Bool(flagAutoApprove, envBool("INPUT_AUTO_APPROVE"), "Apply approved/lgtm labels for eligible patches")

	// Analyze-specific flags
	analyzeCmd.Flags().String(flagProvider, envStr("INPUT_LLM_PROVIDER", "gemini"), "LLM provider (gemini, claude)")
	analyzeCmd.Flags().String(flagAPIKey, envStr("INPUT_API_KEY", ""), "LLM provider API key")
	analyzeCmd.Flags().String(flagModel, envStr("INPUT_LLM_MODEL", ""), "LLM model name (provider-dependent default)")
	analyzeCmd.Flags().Bool(flagAutoApprove, envBool("INPUT_AUTO_APPROVE"), "Enable formal APPROVE review for low-risk patches")
	analyzeCmd.Flags().Bool(flagAutoMerge, envBool("INPUT_AUTO_MERGE"), "Merge eligible PRs after analysis (requires auto-approve)")
	analyzeCmd.Flags().String(flagClassifyOutput, "/tmp/deptriage-classify.json", "Path to classify result JSON")

	// Both command inherits all flags from classify and analyze
	bothCmd.Flags().AddFlagSet(classifyCmd.Flags())
	bothCmd.Flags().AddFlagSet(analyzeCmd.Flags())

	// Merge-specific flags
	mergeCmd.Flags().String("head-sha", envStr("INPUT_HEAD_SHA", ""), "Find and merge open PRs for this head SHA")

	rootCmd.AddCommand(classifyCmd, analyzeCmd, bothCmd, mergeCmd)
}

func runClassify(cmd *cobra.Command, args []string) error {
	autoApprove, _ := cmd.Flags().GetBool(flagAutoApprove)
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)

	result, err := classify.Run(cmd.Context(), classify.Options{
		PRNumber:    prNumber,
		Repo:        repo,
		Token:       githubToken,
		OutputFile:  outputFile,
		AutoApprove: autoApprove,
		DryRun:      dryRun,
	})
	if err != nil {
		return err
	}

	writeGitHubOutput("bump-type", result.BumpType.String())
	writeGitHubOutput("context-json", outputFile)
	return nil
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	providerName, _ := cmd.Flags().GetString(flagProvider)
	apiKey, _ := cmd.Flags().GetString(flagAPIKey)
	model, _ := cmd.Flags().GetString(flagModel)
	autoApprove, _ := cmd.Flags().GetBool(flagAutoApprove)
	autoMerge, _ := cmd.Flags().GetBool(flagAutoMerge)
	classifyOutput, _ := cmd.Flags().GetString(flagClassifyOutput)
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)

	workDir, _ := os.Getwd()

	riskLevel, err := analyze.Run(cmd.Context(), analyze.Options{
		PRNumber:       prNumber,
		Repo:           repo,
		Token:          githubToken,
		Provider:       providerName,
		APIKey:         apiKey,
		Model:          model,
		AutoApprove:    autoApprove,
		AutoMerge:      autoMerge,
		ClassifyOutput: classifyOutput,
		DryRun:         dryRun,
		WorkDir:        workDir,
	})
	// analyze always exits 0
	if err != nil {
		slog.Warn("analyze completed with warning", "error", err)
	}
	writeGitHubOutput("risk-level", riskLevel.String())
	return nil
}

func runBoth(cmd *cobra.Command, args []string) error {
	if err := runClassify(cmd, args); err != nil {
		return err
	}
	_ = cmd.Flags().Set(flagClassifyOutput, outputFile)
	return runAnalyze(cmd, args)
}

func runMerge(cmd *cobra.Command, args []string) error {
	headSHA, _ := cmd.Flags().GetString("head-sha")
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)

	err := merge.Run(cmd.Context(), merge.Options{
		PRNumber: prNumber,
		HeadSHA:  headSHA,
		Repo:     repo,
		Token:    githubToken,
		DryRun:   dryRun,
	})
	if err != nil {
		slog.Warn("merge completed with warning", "error", err)
	}
	return nil
}

// writeGitHubOutput appends a key=value pair to $GITHUB_OUTPUT if running in GitHub Actions.
func writeGitHubOutput(key, value string) {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("failed to write GitHub output", "key", key, "error", err)
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "%s=%s\n", key, value)
}

// Env var helpers for defaulting flags from GitHub Actions INPUT_* variables.

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string) bool {
	return os.Getenv(key) == "true"
}
