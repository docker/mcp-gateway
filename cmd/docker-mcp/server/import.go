package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oci"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func Import(registryUrl string, ociRepository string, push bool) error {
	if registryUrl == "" {
		return fmt.Errorf("registry URL is required")
	}
	if ociRepository == "" {
		return fmt.Errorf("OCI repository is required")
	}

	// Fetch JSON document from registryUrl
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(registryUrl)
	if err != nil {
		return fmt.Errorf("failed to fetch JSON from %s: %w", registryUrl, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JSON: HTTP %d", resp.StatusCode)
	}

	jsonContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JSON content: %w", err)
	}

	// Pretty print the JSON content
	var jsonData interface{}
	if err := json.Unmarshal(jsonContent, &jsonData); err != nil {
		fmt.Printf("Warning: Failed to parse JSON for pretty printing: %v\n", err)
		fmt.Printf("Raw JSON content:\n%s\n", string(jsonContent))
	} else {
		_, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			fmt.Printf("Warning: Failed to format JSON: %v\n", err)
			fmt.Printf("Raw JSON content:\n%s\n", string(jsonContent))
		} 
	}

	// Parse packages from JSON data
	var dockerPackages []map[string]interface{}
	if jsonMap, ok := jsonData.(map[string]interface{}); ok {
		if serverInterface, exists := jsonMap["server"]; exists {
			if serverMap, ok := serverInterface.(map[string]interface{}); ok {
				if packagesInterface, exists := serverMap["packages"]; exists {
					if packages, ok := packagesInterface.([]interface{}); ok {
						// Filter packages with registry_type=docker
						for _, pkg := range packages {
							if pkgMap, ok := pkg.(map[string]interface{}); ok {
								if registryName, exists := pkgMap["registry_type"]; exists {
									if registryName == "docker" {
										dockerPackages = append(dockerPackages, pkgMap)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Create OCI references from filtered packages
	var ociReferences []name.Reference
	for _, pkg := range dockerPackages {
		nameVal, hasName := pkg["identifier"]
		versionVal, hasVersion := pkg["version"]
		
		if !hasName || !hasVersion {
			fmt.Printf("Warning: Package missing name or version: %v\n", pkg)
			continue
		}
		
		nameStr, ok := nameVal.(string)
		if !ok {
			fmt.Printf("Warning: Package name is not a string: %v\n", nameVal)
			continue
		}
		
		versionStr, ok := versionVal.(string)
		if !ok {
			fmt.Printf("Warning: Package version is not a string: %v\n", versionVal)
			continue
		}

		// Create OCI reference using name as registry/repository and version as tag/digest
		refStr := nameStr + ":" + versionStr
		ref, err := name.ParseReference(refStr)
		if err != nil {
			fmt.Printf("Warning: Failed to parse OCI reference %s: %v\n", refStr, err)
			continue
		}
		
		ociReferences = append(ociReferences, ref)
	}

	// Take the first OCI reference and verify it can be resolved
	if len(ociReferences) == 0 {
		return fmt.Errorf("no valid Docker registry packages found")
	}

	firstRef := ociReferences[0]

	// Verify the reference can be resolved
	subjectDescriptor, err := remote.Get(firstRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("failed to resolve reference %s: %w", firstRef.Name(), err)
	}

	// Parse and resolve the OCI repository reference for the artifact
	artifactRef, err := name.ParseReference(ociRepository)
	if err != nil {
		return fmt.Errorf("failed to parse OCI repository reference %s: %w", ociRepository, err)
	}

	// Create the OCI artifact with the subject
	manifest, err := oci.CreateArtifactWithSubjectAndPush(artifactRef, jsonContent, subjectDescriptor.Digest, subjectDescriptor.Size, subjectDescriptor.MediaType, push)
	if err != nil {
		return fmt.Errorf("failed to create OCI artifact: %w", err)
	}

	fmt.Printf("%s@%s", artifactRef.Context().Name(), manifest)

	return nil
}

