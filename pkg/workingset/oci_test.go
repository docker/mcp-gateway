package workingset

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullNameWithDockerHub(t *testing.T) {
	ref, err := name.ParseReference("docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// Docker Hub domain should be stripped
	assert.Equal(t, "library/nginx:latest", fullNameStr)
}

func TestFullNameWithDockerHubIndexDomain(t *testing.T) {
	ref, err := name.ParseReference("index.docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// index.docker.io domain should be stripped
	assert.Equal(t, "library/nginx:latest", fullNameStr)
}

func TestFullNameWithCustomRegistry(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage:v1.0")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// Custom registry domain should be preserved
	assert.Equal(t, "myregistry.example.com/myrepo/myimage:v1.0", fullNameStr)
}

func TestFullNameWithGCR(t *testing.T) {
	ref, err := name.ParseReference("gcr.io/my-project/my-image:latest")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// GCR domain should be preserved
	assert.Equal(t, "gcr.io/my-project/my-image:latest", fullNameStr)
}

func TestFullNameWithDigest(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// Should include digest
	assert.Contains(t, fullNameStr, "@sha256:")
	assert.Contains(t, fullNameStr, "myregistry.example.com/myrepo/myimage")
}

func TestFullNameWithNoTag(t *testing.T) {
	ref, err := name.ParseReference("nginx")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// Docker Hub should be stripped
	// Default tag handling depends on the library
	assert.Contains(t, fullNameStr, "nginx")
}

func TestFullNameWithPort(t *testing.T) {
	ref, err := name.ParseReference("localhost:5000/myimage:v1.0")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	// localhost:5000 should be preserved
	assert.Equal(t, "localhost:5000/myimage:v1.0", fullNameStr)
}

func TestFullNameWithMultiplePathComponents(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/org/team/project/image:tag")
	require.NoError(t, err)

	fullNameStr := fullName(ref)

	assert.Equal(t, "myregistry.example.com/org/team/project/image:tag", fullNameStr)
}

func TestIsValidInputReferenceWithTag(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		expected  bool
	}{
		{
			name:      "valid tag",
			reference: "nginx:latest",
			expected:  true,
		},
		{
			name:      "valid tag with registry",
			reference: "myregistry.example.com/nginx:v1.0",
			expected:  true,
		},
		{
			name:      "implicit latest tag",
			reference: "nginx",
			expected:  true,
		},
		{
			name:      "digest reference",
			reference: "nginx@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := name.ParseReference(tt.reference)
			require.NoError(t, err)

			result := isValidInputReference(ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidInputReferenceRejectsDigests(t *testing.T) {
	digest := "myregistry.example.com/myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	ref, err := name.ParseReference(digest)
	require.NoError(t, err)

	result := isValidInputReference(ref)
	assert.False(t, result, "digest references should not be valid input references")
}

func TestMCPCatalogArtifactType(t *testing.T) {
	assert.Equal(t, "application/vnd.docker.mcp.catalog.v1+json", MCPCatalogArtifactType)
}
