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

package imports

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const modToolTimeout = 60 * time.Second

// ModWhy runs `go mod why -m <module>` and returns the import chain.
// Returns empty string if the module is not needed.
func ModWhy(ctx context.Context, module, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, modToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "mod", "why", "-m", module)
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("go mod why timed out after %s", modToolTimeout)
	}
	if err != nil {
		// go mod why returns non-zero when module is not needed
		output := stdout.String()
		if strings.Contains(output, "(main module does not need") {
			return "", nil
		}
		return "", fmt.Errorf("go mod why: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ModGraph runs `go mod graph` and filters for lines containing the given module.
func ModGraph(ctx context.Context, module, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, modToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "mod", "graph")
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("go mod graph timed out after %s", modToolTimeout)
	}
	if err != nil {
		return "", fmt.Errorf("go mod graph: %w: %s", err, stderr.String())
	}

	var relevant []string
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		if strings.Contains(line, module) {
			relevant = append(relevant, line)
		}
	}
	return strings.Join(relevant, "\n"), nil
}

// GoAvailable checks if the go binary and go.mod are available in workDir.
func GoAvailable(workDir string) bool {
	if _, err := exec.LookPath("go"); err != nil {
		return false
	}
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
