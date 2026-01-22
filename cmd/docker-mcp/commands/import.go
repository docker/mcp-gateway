package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/oci"
)

func runMcpregistryImport(ctx context.Context, serverURL string, servers *[]catalog.Server) error {
	// Validate URL
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https protocol")
	}

	// Fetch the server definition
	fmt.Printf("Fetching server definition from: %s\n\n", serverURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Transport: desktop.ProxyTransport(),
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch server definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch server definition: HTTP %d %s", resp.StatusCode, resp.Status)
	}

	// Parse the JSON response
	var serverDetail oci.ServerDetail
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&serverDetail); err != nil {
		return fmt.Errorf("failed to parse server definition: %w", err)
	}

	// Convert to catalog server
	catalogServer := serverDetail.ToCatalogServer()

	// Add to servers slice if provided (for gateway use)
	if servers != nil {
		*servers = append(*servers, catalogServer)
	} else {
		// Pretty print the results (for import command use)
		fmt.Println("=== Server Detail (Original) ===")
		serverDetailJSON, err := json.MarshalIndent(serverDetail, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal server detail: %w", err)
		}
		fmt.Println(string(serverDetailJSON))

		fmt.Println("\n=== Catalog Server (Converted) ===")
		catalogServerJSON, err := json.MarshalIndent(catalogServer, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal catalog server: %w", err)
		}
		fmt.Println(string(catalogServerJSON))

		// Print summary information
		fmt.Println("\n=== Summary ===")
		fmt.Printf("Server Name: %s\n", serverDetail.Name)
		fmt.Printf("Description: %s\n", serverDetail.Description)
		fmt.Printf("Status: %s\n", serverDetail.Status)

		if serverDetail.VersionDetail != nil {
			fmt.Printf("Version: %s\n", serverDetail.VersionDetail.Version)
		}

		if len(serverDetail.Packages) > 0 {
			pkg := serverDetail.Packages[0]
			fmt.Printf("Registry Type: %s\n", pkg.RegistryType)
			fmt.Printf("Image: %s:%s\n", pkg.Identifier, pkg.Version)
			fmt.Printf("Environment Variables: %d\n", len(pkg.Env))
		}

		fmt.Printf("Secrets Required: %d\n", len(catalogServer.Secrets))
		if len(catalogServer.Secrets) > 0 {
			fmt.Printf("Secret Names: ")
			for i, secret := range catalogServer.Secrets {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", secret.Name)
			}
			fmt.Println()
		}

		fmt.Printf("Config Schemas: %d\n", len(catalogServer.Config))
	}

	return nil
}
