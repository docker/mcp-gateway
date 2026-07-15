package commands

import (
	"testing"

	seclient "github.com/docker/secrets-engine/client"
	"github.com/stretchr/testify/assert"
)

// TestFilterAllDockerMCP verifies that `rm --all` only targets docker/mcp/** IDs
// (including oauth/oauth-dcr) and excludes unrelated docker pass keys such as
// docker/auth/**, docker/sandbox/** or arbitrary app keys.
func TestFilterAllDockerMCP(t *testing.T) {
	ids := []seclient.ID{
		seclient.MustParseID("docker/mcp/foo"),
		seclient.MustParseID("docker/mcp/oauth/github"),
		seclient.MustParseID("docker/mcp/oauth-dcr/github"),
		seclient.MustParseID("docker/auth/hub/user"),
		seclient.MustParseID("docker/sandbox/thing"),
		seclient.MustParseID("myapp/key"),
	}

	var got []string
	for _, id := range filterAllDockerMCP(ids) {
		got = append(got, id.String())
	}

	assert.Equal(t, []string{
		"docker/mcp/foo",
		"docker/mcp/oauth/github",
		"docker/mcp/oauth-dcr/github",
	}, got)
}
