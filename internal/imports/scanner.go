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
	m, err := ScanImportsForPackages(rootDir, []string{pkg})
	if err != nil {
		return nil, err
	}
	return m[pkg], nil
}

// ScanImportsForPackages walks rootDir once and collects import usage for each
// package path in packages. Duplicate names are deduplicated. The returned map has
// an entry only for packages that had at least one matching file.
func ScanImportsForPackages(rootDir string, packages []string) (map[string][]types.ImportInfo, error) {
	out := make(map[string][]types.ImportInfo)
	uniq := dedupePackages(packages)
	if len(uniq) == 0 {
		return out, nil
	}

	type pkgMatch struct {
		name   string
		quoted string
	}
	matches := make([]pkgMatch, 0, len(uniq))
	for _, p := range uniq {
		matches = append(matches, pkgMatch{name: p, quoted: fmt.Sprintf("%q", p)})
	}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip errors
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == "hack" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		lines, err := readLines(path)
		if err != nil {
			return nil
		}

		var matched []pkgMatch
		for _, m := range matches {
			if fileContainsImportLines(lines, m.quoted) {
				matched = append(matched, m)
			}
		}
		if len(matched) == 0 {
			return nil
		}

		pkgNames := make([]string, len(matched))
		for i, m := range matched {
			pkgNames[i] = m.name
		}
		snippetsByPkg := extractSnippetsForPackages(lines, pkgNames)

		relPath, _ := filepath.Rel(rootDir, path)
		if relPath == "" {
			relPath = path
		}
		hasTest := testFileExists(path)
		for _, m := range matched {
			out[m.name] = append(out[m.name], types.ImportInfo{
				File:    relPath,
				HasTest: hasTest,
				Snippet: snippetsByPkg[m.name],
			})
		}
		return nil
	})

	return out, err
}

func dedupePackages(packages []string) []string {
	seen := make(map[string]struct{}, len(packages))
	out := make([]string, 0, len(packages))
	for _, p := range packages {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func fileContainsImportLines(lines []string, quoted string) bool {
	for _, line := range lines {
		if strings.Contains(line, quoted) {
			return true
		}
	}
	return false
}

// extractSnippetsForPackages builds snippet text for each pkg in one pass over lines.
// Keys are omitted when a package has no matching lines (caller sees "").
func extractSnippetsForPackages(lines []string, pkgs []string) map[string]string {
	if len(pkgs) == 0 {
		return nil
	}
	blocks := make(map[string][]string, len(pkgs))
	for _, pkg := range pkgs {
		blocks[pkg] = nil
	}
	for i, line := range lines {
		for _, pkg := range pkgs {
			if strings.Contains(line, pkg) {
				start := max(i-snippetContextLines, 0)
				end := min(i+snippetContextLines+1, len(lines))
				block := strings.Join(lines[start:end], "\n")
				blocks[pkg] = append(blocks[pkg], block)
			}
		}
	}
	out := make(map[string]string, len(pkgs))
	for _, pkg := range pkgs {
		if b := blocks[pkg]; len(b) > 0 {
			out[pkg] = strings.Join(b, "\n---\n")
		}
	}
	return out
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

	if _, err := os.Stat(filepath.Join(dir, name+"_test.go")); err == nil {
		return true
	}

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
