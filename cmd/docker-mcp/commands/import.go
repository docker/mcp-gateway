package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/remoteurl"
)

func runMcpregistryImport(ctx context.Context, serverURL string, servers *[]catalog.Server) error {
	serverURL = remoteurl.UpgradeKnownHTTPURLToHTTPS(serverURL)

	if err := remoteurl.Validate(ctx, serverURL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch the server definition
	fmt.Printf("Fetching server definition from: %s\n\n", serverURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := remoteurl.NewDirectHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch server definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch server definition: HTTP %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read server definition: %w", err)
	}

	details, isCollection, err := parseServerDetails(body)
	if err != nil {
		return fmt.Errorf("failed to parse server definition: %w", err)
	}
	if isCollection && len(details) == 0 {
		return fmt.Errorf("no servers found at MCP registry URL")
	}

	for _, serverDetail := range details {
		if serverDetail.Name == "" {
			continue
		}

		catalogServer := serverDetail.ToCatalogServer()
		if servers != nil {
			*servers = append(*servers, catalogServer)
		} else if err := printServerDetail(serverDetail, catalogServer); err != nil {
			return err
		}
	}

	return nil
}

// parseServerDetails accepts both a single server detail and the paged
// envelope returned by registry.modelcontextprotocol.io (`{"servers":[{"server":...}]}`).
func parseServerDetails(data []byte) ([]oci.ServerDetail, bool, error) {
	var envelope struct {
		Servers json.RawMessage `json:"servers"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false, err
	}
	if len(envelope.Servers) > 0 && envelope.Servers[0] == '[' {
		var entries []struct {
			Server oci.ServerDetail `json:"server"`
		}
		if err := json.Unmarshal(envelope.Servers, &entries); err != nil {
			return nil, true, err
		}
		details := make([]oci.ServerDetail, 0, len(entries))
		for _, entry := range entries {
			details = append(details, entry.Server)
		}
		return details, true, nil
	}

	var serverDetail oci.ServerDetail
	if err := json.Unmarshal(data, &serverDetail); err != nil {
		return nil, false, err
	}
	return []oci.ServerDetail{serverDetail}, false, nil
}

func printServerDetail(serverDetail oci.ServerDetail, catalogServer catalog.Server) error {
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
	return nil
}
