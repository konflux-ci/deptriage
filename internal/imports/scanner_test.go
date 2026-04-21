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
	"os"
	"path/filepath"
	"testing"
)

func TestScanImports(t *testing.T) {
	// Create a temp directory with test Go files
	dir := t.TempDir()

	// File that imports the package
	writeTestFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"github.com/foo/bar"
)

func main() {
	bar.DoSomething()
	fmt.Println("hello")
}
`)

	// Test file (should be excluded)
	writeTestFile(t, filepath.Join(dir, "main_test.go"), `package main

import "github.com/foo/bar"

func TestSomething(t *testing.T) {
	bar.DoSomething()
}
`)

	// File that doesn't import the package
	writeTestFile(t, filepath.Join(dir, "other.go"), `package main

import "fmt"

func other() {
	fmt.Println("no import")
}
`)

	results, err := ScanImports(dir, "github.com/foo/bar")
	if err != nil {
		t.Fatalf("ScanImports() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].File != "main.go" {
		t.Errorf("expected file main.go, got %s", results[0].File)
	}

	if !results[0].HasTest {
		t.Error("expected hasTest=true (main_test.go exists)")
	}

	if results[0].Snippet == "" {
		t.Error("expected non-empty snippet")
	}
}

func TestScanImports_NoResults(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("hello") }
`)

	results, err := ScanImports(dir, "github.com/nonexistent/pkg")
	if err != nil {
		t.Fatalf("ScanImports() error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}
