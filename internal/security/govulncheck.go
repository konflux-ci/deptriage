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

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

const govulncheckTimeout = 300 * time.Second

// govulncheckFinding represents a single finding from govulncheck JSON output.
type govulncheckFinding struct {
	OSV   string `json:"osv"`
	Trace []struct {
		Module   string `json:"module,omitempty"`
		Package  string `json:"package,omitempty"`
		Function string `json:"function,omitempty"`
	} `json:"trace"`
}

type govulncheckMessage struct {
	Finding *govulncheckFinding `json:"finding,omitempty"`
}

// RunGovulncheck runs govulncheck in the given working directory and returns results.
func RunGovulncheck(ctx context.Context, workDir string) (*types.GovulncheckResult, error) {
	if _, err := exec.LookPath("govulncheck"); err != nil {
		return nil, nil // not installed, skip gracefully
	}

	ctx, cancel := context.WithTimeout(ctx, govulncheckTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "govulncheck", "-json", "./...")
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("govulncheck timed out after %s", govulncheckTimeout)
	}
	// govulncheck exits non-zero when vulnerabilities are found, which is expected
	if err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("govulncheck failed: %w: %s", err, stderr.String())
	}

	return parseGovulncheckOutput(stdout.String())
}

func parseGovulncheckOutput(output string) (*types.GovulncheckResult, error) {
	result := &types.GovulncheckResult{}

	// govulncheck -json outputs newline-delimited JSON messages
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg govulncheckMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Finding == nil {
			continue
		}

		finding := types.VulnFinding{
			ID: msg.Finding.OSV,
		}

		// Build call chain from trace
		var chain []string
		for _, t := range msg.Finding.Trace {
			if t.Function != "" {
				chain = append(chain, t.Function)
				finding.Symbol = t.Function
			}
		}
		if len(chain) > 0 {
			finding.CallChain = strings.Join(chain, " -> ")
			result.Reachable = true
		}

		result.Findings = append(result.Findings, finding)
	}

	return result, nil
}
