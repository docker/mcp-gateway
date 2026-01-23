package gateway

import (
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/catalog"
)

// ServerSecretsInput represents info needed to build secrets URIs
type ServerSecretsInput struct {
	Secrets        []catalog.Secret // Secret definitions from server catalog
	OAuth          *catalog.OAuth   // Reserved for future OAuth priority handling
	ProviderPrefix string           // Optional prefix for map keys (WorkingSet namespacing)
}

// BuildSecretsURIs generates se:// URIs for all defined secrets.
// Does NOT call GetSecrets - URIs are built for all secrets.
// Docker Desktop resolves se:// URIs at container runtime.
// Remote servers fetch actual values via remote.go.
func BuildSecretsURIs(inputs []ServerSecretsInput) map[string]string {
	uris := make(map[string]string)

	for _, input := range inputs {
		for _, s := range input.Secrets {
			key := secret.GetDefaultSecretKey(s.Name)
			mapKey := input.ProviderPrefix + s.Name
			uris[mapKey] = "se://" + key
		}
	}
	return uris
}
