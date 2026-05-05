// npm-catalog-check fetches all npm-only servers from the MCP community registry
// and runs TransformToDocker on each one, reporting pass/fail/skip counts.
//
// Usage:
//
//	go run ./scripts/npm-catalog-check                  # all npm servers (no limit)
//	go run ./scripts/npm-catalog-check -limit 20        # first 20 npm servers
//	go run ./scripts/npm-catalog-check -verbose          # print each server result
//	go run ./scripts/npm-catalog-check -retries 5        # retry failed page fetches up to 5 times
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

type registryListResponse struct {
	Servers []struct {
		Server v0.ServerJSON `json:"server"`
	} `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

func main() {
	limit := flag.Int("limit", 0, "max npm servers to check (0 = unlimited)")
	verbose := flag.Bool("verbose", false, "print result for each server")
	maxRetries := flag.Int("retries", 5, "max retries per page fetch on timeout/network error")
	pageTimeout := flag.Duration("page-timeout", 60*time.Second, "HTTP timeout per page request")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// Explicit transport bypassing macOS system/Desktop proxy settings.
	httpClient := &http.Client{
		Timeout: *pageTimeout,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{},
			DialContext:         (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout: 15 * time.Second,
			// Keep connections alive across retries and pages.
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	// Also use a direct-connection resolver for npm version lookups.
	npmResolver := catalog.NewNPMVersionResolver(httpClient)

	fmt.Println("Fetching npm-only servers from community registry...")
	fmt.Printf("Settings: page-timeout=%s, retries=%d\n", *pageTimeout, *maxRetries)

	var (
		passed       int
		failed       int
		skipped      int
		total        int
		failedNames  []string
		skippedNames []string
	)

	cursor := ""
	baseURL := "https://registry.modelcontextprotocol.io/v0/servers?version=latest&limit=100"
	pagesRead := 0
	done := false

	for !done {
		reqURL := baseURL
		if cursor != "" {
			reqURL += "&cursor=" + url.QueryEscape(cursor)
		}

		body, err := fetchWithRetry(ctx, httpClient, reqURL, pagesRead+1, *maxRetries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Giving up on page %d after %d retries: %v\n", pagesRead+1, *maxRetries, err)
			break
		}

		var listResp registryListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing page %d: %v\n", pagesRead+1, err)
			break
		}

		pagesRead++

		for _, s := range listResp.Servers {
			// Filter to npm-only stdio servers (no OCI, no PyPI).
			pkgs := s.Server.Packages
			hasNPMStdio := false
			hasOCI := false
			hasPyPI := false
			for _, p := range pkgs {
				switch p.RegistryType {
				case "oci":
					hasOCI = true
				case "pypi":
					hasPyPI = true
				case "npm":
					if p.Transport.Type == "stdio" {
						hasNPMStdio = true
					}
				}
			}
			if !hasNPMStdio || hasOCI || hasPyPI {
				continue
			}

			total++
			name := s.Server.Name

			if *limit > 0 && total > *limit {
				done = true
				total-- // don't count the one we skipped
				break
			}

			result, source, err := catalog.TransformToDocker(ctx, s.Server,
				catalog.WithNPMResolver(npmResolver),
			)
			if err != nil {
				skipped++
				skippedNames = append(skippedNames, fmt.Sprintf("%s: %v", name, err))
				if *verbose {
					fmt.Printf("  SKIP  %-60s %v\n", name, err)
				}
				continue
			}

			if source != catalog.TransformSourceNPM {
				skipped++
				skippedNames = append(skippedNames, fmt.Sprintf("%s: resolved as %s", name, source))
				if *verbose {
					fmt.Printf("  SKIP  %-60s resolved as %s\n", name, source)
				}
				continue
			}

			// Validate the transform output.
			var problems []string

			if !strings.HasPrefix(result.Image, "node:") {
				problems = append(problems, fmt.Sprintf("image %q doesn't start with node:", result.Image))
			}
			if !strings.HasSuffix(result.Image, "-bookworm-slim") {
				problems = append(problems, fmt.Sprintf("image %q doesn't end with -bookworm-slim", result.Image))
			}
			if len(result.Command) < 3 {
				problems = append(problems, fmt.Sprintf("command too short: %v", result.Command))
			} else {
				if result.Command[0] != "npx" {
					problems = append(problems, fmt.Sprintf("command[0]=%q, want npx", result.Command[0]))
				}
				if result.Command[1] != "--yes" {
					problems = append(problems, fmt.Sprintf("command[1]=%q, want --yes", result.Command[1]))
				}
			}
			if result.Type != "server" {
				problems = append(problems, fmt.Sprintf("type=%q, want server", result.Type))
			}
			if !result.LongLived {
				problems = append(problems, "longLived=false, want true")
			}

			hasNPMCache := false
			for _, v := range result.Volumes {
				if strings.Contains(v, ":/root/.npm") && strings.HasPrefix(v, "docker-mcp-npm-cache-") {
					hasNPMCache = true
					break
				}
			}
			if !hasNPMCache {
				problems = append(problems, fmt.Sprintf("no npm cache volume in %v", result.Volumes))
			}

			if result.Metadata == nil || result.Metadata.RegistryURL == "" {
				problems = append(problems, "missing metadata.registryUrl")
			}

			if len(problems) > 0 {
				failed++
				failedNames = append(failedNames, fmt.Sprintf("%s: %s", name, strings.Join(problems, "; ")))
				if *verbose {
					fmt.Printf("  FAIL  %-60s %s\n", name, strings.Join(problems, "; "))
				}
			} else {
				passed++
				if *verbose {
					fmt.Printf("  PASS  %-60s %s %v\n", name, result.Image, result.Command)
				}
			}

			// Progress indicator every 100 servers.
			if total%100 == 0 {
				fmt.Printf("  ... checked %d npm servers so far (%d passed, %d failed, %d skipped)\n",
					total, passed, failed, skipped)
			}
		}

		if listResp.Metadata.NextCursor == "" || len(listResp.Servers) == 0 {
			break
		}
		cursor = listResp.Metadata.NextCursor

		fmt.Printf("  Page %d done (%d npm servers found so far)\n", pagesRead, total)
	}

	// Print summary.
	fmt.Println()
	fmt.Println("=== NPM Catalog Check Results ===")
	fmt.Printf("Pages read:            %d\n", pagesRead)
	fmt.Printf("Total npm servers:     %d\n", total)
	fmt.Printf("Passed:                %d\n", passed)
	fmt.Printf("Failed:                %d\n", failed)
	fmt.Printf("Skipped (remote/err):  %d\n", skipped)

	if passed+failed > 0 {
		rate := float64(passed) / float64(passed+failed) * 100
		fmt.Printf("Success rate:          %.1f%% (%d/%d transformable)\n", rate, passed, passed+failed)
	}

	if len(failedNames) > 0 {
		fmt.Println()
		fmt.Println("--- Failed servers ---")
		for _, f := range failedNames {
			fmt.Printf("  %s\n", f)
		}
	}

	if len(skippedNames) > 0 && *verbose {
		fmt.Println()
		fmt.Println("--- Skipped servers ---")
		for _, s := range skippedNames {
			fmt.Printf("  %s\n", s)
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

// fetchWithRetry fetches a URL with exponential backoff on timeout/network errors.
// Returns the response body on success, or the last error after all retries are exhausted.
func fetchWithRetry(ctx context.Context, client *http.Client, reqURL string, pageNum int, maxRetries int) ([]byte, error) {
	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 5 * time.Second
			fmt.Printf("  Page %d: retry %d/%d after %s...\n", pageNum, attempt, maxRetries, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetching page %d: %w", pageNum, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading page %d: %w", pageNum, err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("page %d returned %d: %s", pageNum, resp.StatusCode, truncate(string(body), 200))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("page %d returned %d: %s", pageNum, resp.StatusCode, truncate(string(body), 200))
		}

		return body, nil
	}
	return nil, lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
