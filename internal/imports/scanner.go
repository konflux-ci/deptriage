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

package imports

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konflux-ci/deptriage/internal/types"
)

const snippetContextLines = 5

// ScanImports scans Go source files in rootDir for imports of the given package.
// Excludes test files, vendor/, and hack/ directories.
func ScanImports(rootDir, pkg string) ([]types.ImportInfo, error) {
	var results []types.ImportInfo

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip directories
		if d.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == "hack" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only .go files, exclude tests
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		if fileContainsImport(path, pkg) {
			relPath, _ := filepath.Rel(rootDir, path)
			if relPath == "" {
				relPath = path
			}
			snippet := extractSnippets(path, pkg)
			hasTest := testFileExists(path)
			results = append(results, types.ImportInfo{
				File:    relPath,
				HasTest: hasTest,
				Snippet: snippet,
			})
		}
		return nil
	})

	return results, err
}

func fileContainsImport(path, pkg string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	quoted := fmt.Sprintf("%q", pkg)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), quoted) {
			return true
		}
	}
	return false
}

func extractSnippets(path, pkg string) string {
	lines, err := readLines(path)
	if err != nil {
		return ""
	}

	var snippets []string
	for i, line := range lines {
		if strings.Contains(line, pkg) {
			start := max(i-snippetContextLines, 0)
			end := min(i+snippetContextLines+1, len(lines))
			snippet := strings.Join(lines[start:end], "\n")
			snippets = append(snippets, snippet)
		}
	}
	return strings.Join(snippets, "\n---\n")
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func testFileExists(goFile string) bool {
	dir := filepath.Dir(goFile)
	base := filepath.Base(goFile)
	name := strings.TrimSuffix(base, ".go")

	// Check specific test file
	if _, err := os.Stat(filepath.Join(dir, name+"_test.go")); err == nil {
		return true
	}

	// Check any test files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			return true
		}
	}
	return false
}
