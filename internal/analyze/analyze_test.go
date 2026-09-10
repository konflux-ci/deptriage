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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRemovesStaleReportWhenInspectionFails(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	if err := os.WriteFile(report, []byte(`{"stale":true}`), 0644); err != nil {
		t.Fatalf("writing stale report: %v", err)
	}

	err := Run(context.Background(), Options{
		ClassifyOutput: filepath.Join(dir, "missing-classify.json"),
		ContextOutput:  report,
		WorkDir:        dir,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if _, statErr := os.Stat(report); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("stale report still exists or could not be checked: %v", statErr)
	}
}
