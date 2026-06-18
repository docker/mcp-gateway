// npm-catalog-count does a raw count of all servers in the MCP community registry,
// broken down by package type, to reconcile against the context doc's numbers.
//
// Usage: go run ./scripts/npm-catalog-count/
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type registryListResponse struct {
	Servers []struct {
		Server serverJSON `json:"server"`
	} `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

type serverJSON struct {
	Name     string          `json:"name"`
	Packages []pkgJSON       `json:"packages"`
	Remotes  []remoteJSON    `json:"remotes"`
}

type remoteJSON struct {
	Type string `json:"type"`
}

type pkgJSON struct {
	RegistryType string `json:"registryType"`
	Transport    struct {
		Type string `json:"type"`
	} `json:"transport"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{},
			DialContext:         (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}

	cursor := ""
	baseURL := "https://registry.modelcontextprotocol.io/v0/servers?version=latest&limit=100"
	pagesRead := 0
	totalServers := 0

	// Per-server counts (a server can appear in multiple categories).
	var (
		hasNPM        int // server has at least one npm package
		hasNPMStdio   int // server has at least one npm+stdio package
		hasNPMRemote  int // server has at least one npm+non-stdio package
		hasOCI        int
		hasPyPI       int
		hasRemote     int // server has a non-npm/oci/pypi package or remote transport
		npmOnly       int // npm+stdio, no OCI, no PyPI
		npmOnlyStrict int // npm+stdio, no OCI, no PyPI, no remote/sse transport on any package
	)

	for {
		reqURL := baseURL
		if cursor != "" {
			reqURL += "&cursor=" + url.QueryEscape(cursor)
		}

		body, err := fetchWithRetry(ctx, client, reqURL, pagesRead+1, 5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed page %d: %v\n", pagesRead+1, err)
			break
		}

		var listResp registryListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			fmt.Fprintf(os.Stderr, "Parse error page %d: %v\n", pagesRead+1, err)
			break
		}

		pagesRead++

		for _, s := range listResp.Servers {
			totalServers++

			serverHasNPM := false
			serverHasNPMStdio := false
			serverHasNPMRemote := false
			serverHasOCI := false
			serverHasPyPI := false
			serverHasRemote := false

			for _, p := range s.Server.Packages {
				switch p.RegistryType {
				case "npm":
					serverHasNPM = true
					if p.Transport.Type == "stdio" {
						serverHasNPMStdio = true
					} else {
						serverHasNPMRemote = true
					}
				case "oci":
					serverHasOCI = true
				case "pypi":
					serverHasPyPI = true
				default:
					serverHasRemote = true
				}
				// Also count non-stdio transports as "remote-like"
				if p.Transport.Type != "stdio" && p.Transport.Type != "" {
					serverHasRemote = true
				}
			}

			// Count remotes field too.
			if len(s.Server.Remotes) > 0 {
				serverHasRemote = true
			}

			if serverHasNPM {
				hasNPM++
			}
			if serverHasNPMStdio {
				hasNPMStdio++
			}
			if serverHasNPMRemote {
				hasNPMRemote++
			}
			if serverHasOCI {
				hasOCI++
			}
			if serverHasPyPI {
				hasPyPI++
			}
			if serverHasRemote {
				hasRemote++
			}
			if serverHasNPMStdio && !serverHasOCI && !serverHasPyPI {
				npmOnly++
			}
			if serverHasNPMStdio && !serverHasOCI && !serverHasPyPI && !serverHasRemote {
				npmOnlyStrict++
			}
		}

		if pagesRead%10 == 0 {
			fmt.Printf("  ... page %d, %d servers so far\n", pagesRead, totalServers)
		}

		if listResp.Metadata.NextCursor == "" || len(listResp.Servers) == 0 {
			break
		}
		cursor = listResp.Metadata.NextCursor
	}

	fmt.Println()
	fmt.Println("=== Community Registry Breakdown ===")
	fmt.Printf("Pages read:                          %d\n", pagesRead)
	fmt.Printf("Total servers:                       %d\n", totalServers)
	fmt.Println()
	fmt.Println("--- Servers with package type ---")
	fmt.Printf("Has npm (any transport):             %d  (%.1f%%)\n", hasNPM, pct(hasNPM, totalServers))
	fmt.Printf("Has npm+stdio:                       %d  (%.1f%%)\n", hasNPMStdio, pct(hasNPMStdio, totalServers))
	fmt.Printf("Has npm+non-stdio (SSE/streaming):   %d  (%.1f%%)\n", hasNPMRemote, pct(hasNPMRemote, totalServers))
	fmt.Printf("Has OCI:                             %d  (%.1f%%)\n", hasOCI, pct(hasOCI, totalServers))
	fmt.Printf("Has PyPI:                            %d  (%.1f%%)\n", hasPyPI, pct(hasPyPI, totalServers))
	fmt.Printf("Has remote/SSE transport:            %d  (%.1f%%)\n", hasRemote, pct(hasRemote, totalServers))
	fmt.Println()
	fmt.Println("--- npm-only (our target) ---")
	fmt.Printf("npm+stdio, no OCI, no PyPI:          %d  (script filter)\n", npmOnly)
	fmt.Printf("npm+stdio, no OCI/PyPI/remote:       %d  (strict: no remote fallback)\n", npmOnlyStrict)
	fmt.Println()
	fmt.Printf("Context doc claimed:                 8387 npm total, 8167 npm-only\n")
	fmt.Printf("Delta (total npm):                   %d\n", hasNPM-8387)
	fmt.Printf("Delta (npm-only):                    %d\n", npmOnly-8167)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

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
			lastErr = fmt.Errorf("page %d returned %d: %s", pageNum, resp.StatusCode, string(body[:min(200, len(body))]))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("page %d returned %d: %s", pageNum, resp.StatusCode, string(body[:min(200, len(body))]))
		}

		return body, nil
	}
	return nil, lastErr
}
