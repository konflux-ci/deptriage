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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/konflux-ci/deptriage/internal/types"
)

// Exercise NoDirectImports vs source scan: go mod why reports no chain when the
// module is not required in go.mod, but the scanner still sees the import line.
func TestGatherContext_ScanClearsNoDirectImportsWhenModWhyChainEmpty(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	dir := t.TempDir()
	const badModule = "github.com/evil/unused"

	writeCtxTestFile(t, filepath.Join(dir, "go.mod"), `module example.com/gatherctxtest

go 1.21
`)
	writeCtxTestFile(t, filepath.Join(dir, "main.go"), `package main

import _ "`+badModule+`"

func main() {}
`)

	result := &types.ClassifyResult{
		Packages: []types.PackageInfo{{Name: badModule}},
	}

	ctxJSON := GatherContext(context.Background(), result, nil, nil, dir)
	if len(ctxJSON.Packages) != 1 {
		t.Fatalf("packages: got %d want 1", len(ctxJSON.Packages))
	}

	pc := ctxJSON.Packages[0]
	if pc.NoDirectImports {
		t.Errorf("NoDirectImports must be false when source scan finds %q (contradicts empty mod why chain)", badModule)
	}
	if len(pc.Imports) == 0 {
		t.Errorf("expected source scan to record import usage for %q", badModule)
	}
}

func writeCtxTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
