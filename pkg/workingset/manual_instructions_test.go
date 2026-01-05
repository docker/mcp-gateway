package workingset

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteManualInstructions_HumanReadable(t *testing.T) {
	var buf bytes.Buffer

	err := WriteManualInstructions("my-profile", OutputFormatHumanReadable, &buf)

	require.NoError(t, err)
	assert.Equal(t, "docker mcp gateway run --profile my-profile", buf.String())
}

func TestWriteManualInstructions_JSON(t *testing.T) {
	var buf bytes.Buffer

	err := WriteManualInstructions("my-profile", OutputFormatJSON, &buf)

	require.NoError(t, err)
	assert.JSONEq(t, `["docker","mcp","gateway","run","--profile","my-profile"]`, buf.String())
}

func TestWriteManualInstructions_YAML(t *testing.T) {
	var buf bytes.Buffer

	err := WriteManualInstructions("my-profile", OutputFormatYAML, &buf)

	require.NoError(t, err)
	expected := `- docker
- mcp
- gateway
- run
- --profile
- my-profile
`
	assert.Equal(t, expected, buf.String())
}

func TestWriteManualInstructions_EmptyProfileID(t *testing.T) {
	var buf bytes.Buffer

	err := WriteManualInstructions("", OutputFormatHumanReadable, &buf)

	require.Error(t, err)
	assert.Equal(t, "profile ID is required", err.Error())
}
