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

package provider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const maxRetries = 3

// doWithRetry executes an HTTP request with exponential backoff on 429 responses.
// The buildReq function is called on each attempt to create a fresh request body.
func doWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error)) (*http.Response, []byte, error) {
	backoff := 5 * time.Second

	for attempt := range maxRetries {
		req, err := buildReq()
		if err != nil {
			return nil, nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, body, nil
		}

		if attempt == maxRetries-1 {
			return resp, body, nil
		}

		wait := backoff
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				wait = time.Duration(secs) * time.Second
			}
		}

		slog.Warn("rate limited, retrying", "attempt", attempt+1, "backoff", wait)

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(wait):
		}

		backoff *= 2
	}

	return nil, nil, fmt.Errorf("unreachable")
}
