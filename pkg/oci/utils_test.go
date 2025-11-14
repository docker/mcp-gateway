package oci

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullNameWithDockerHub(t *testing.T) {
	ref, err := name.ParseReference("docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// Docker Hub domain should be stripped
	assert.Equal(t, "nginx:latest", fullNameStr)
}

func TestFullNameWithDockerHubIndexDomain(t *testing.T) {
	ref, err := name.ParseReference("index.docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// index.docker.io domain should be stripped
	assert.Equal(t, "nginx:latest", fullNameStr)
}

func TestFullNameWithCustomRegistry(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage:v1.0")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// Custom registry domain should be preserved
	assert.Equal(t, "myregistry.example.com/myrepo/myimage:v1.0", fullNameStr)
}

func TestFullNameWithGCR(t *testing.T) {
	ref, err := name.ParseReference("gcr.io/my-project/my-image:latest")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// GCR domain should be preserved
	assert.Equal(t, "gcr.io/my-project/my-image:latest", fullNameStr)
}

func TestFullNameWithDigest(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// Should include digest
	assert.Contains(t, fullNameStr, "@sha256:")
	assert.Contains(t, fullNameStr, "myregistry.example.com/myrepo/myimage")
}

func TestFullNameWithNoTag(t *testing.T) {
	ref, err := name.ParseReference("nginx")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// Docker Hub should be stripped
	// Default tag handling depends on the library
	assert.Contains(t, fullNameStr, "nginx")
}

func TestFullNameWithPort(t *testing.T) {
	ref, err := name.ParseReference("localhost:5000/myimage:v1.0")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

	// localhost:5000 should be preserved
	assert.Equal(t, "localhost:5000/myimage:v1.0", fullNameStr)
}

func TestFullNameWithMultiplePathComponents(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/org/team/project/image:tag")
	require.NoError(t, err)

	fullNameStr := FullName(ref)

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

			result := IsValidInputReference(ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidInputReferenceRejectsDigests(t *testing.T) {
	digest := "myregistry.example.com/myimage@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	ref, err := name.ParseReference(digest)
	require.NoError(t, err)

	result := IsValidInputReference(ref)
	assert.False(t, result, "digest references should not be valid input references")
}

func TestFullNameWithoutDigestWithTag(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage:v1.0")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Should preserve the tag
	assert.Equal(t, "myregistry.example.com/myrepo/myimage:v1.0", fullNameStr)
}

func TestFullNameWithoutDigestWithDockerHub(t *testing.T) {
	ref, err := name.ParseReference("docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Docker Hub domain should be stripped
	assert.Equal(t, "nginx:latest", fullNameStr)
}

func TestFullNameWithoutDigestWithDockerHubIndexDomain(t *testing.T) {
	ref, err := name.ParseReference("index.docker.io/library/nginx:latest")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// index.docker.io domain should be stripped
	assert.Equal(t, "nginx:latest", fullNameStr)
}

func TestFullNameWithoutDigestStripsDigest(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/myrepo/myimage@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Should convert digest to :latest tag
	assert.Equal(t, "myregistry.example.com/myrepo/myimage:latest", fullNameStr)
	assert.NotContains(t, fullNameStr, "@sha256:")
	assert.NotContains(t, fullNameStr, "1234567890abcdef")
}

func TestFullNameWithoutDigestStripsDigestDockerHub(t *testing.T) {
	ref, err := name.ParseReference("docker.io/library/nginx@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Should strip Docker Hub domain and convert digest to :latest
	assert.Equal(t, "nginx:latest", fullNameStr)
	assert.NotContains(t, fullNameStr, "@sha256:")
}

func TestFullNameWithoutDigestWithNoTag(t *testing.T) {
	ref, err := name.ParseReference("nginx")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Should add :latest tag
	assert.Equal(t, "nginx:latest", fullNameStr)
}

func TestFullNameWithoutDigestWithCustomRegistry(t *testing.T) {
	ref, err := name.ParseReference("gcr.io/my-project/my-image:v2.0")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Custom registry should be preserved with tag
	assert.Equal(t, "gcr.io/my-project/my-image:v2.0", fullNameStr)
}

func TestFullNameWithoutDigestWithPort(t *testing.T) {
	ref, err := name.ParseReference("localhost:5000/myimage:dev")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// localhost:5000 should be preserved
	assert.Equal(t, "localhost:5000/myimage:dev", fullNameStr)
}

func TestFullNameWithoutDigestWithPortAndNoTag(t *testing.T) {
	ref, err := name.ParseReference("localhost:5000/myimage")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	// Should add :latest tag
	assert.Equal(t, "localhost:5000/myimage:latest", fullNameStr)
}

func TestFullNameWithoutDigestWithMultiplePathComponents(t *testing.T) {
	ref, err := name.ParseReference("myregistry.example.com/org/team/project/image:tag")
	require.NoError(t, err)

	fullNameStr := FullNameWithoutDigest(ref)

	assert.Equal(t, "myregistry.example.com/org/team/project/image:tag", fullNameStr)
}
