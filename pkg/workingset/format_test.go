package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()

	assert.Len(t, formats, 3)
	assert.Contains(t, formats, "json")
	assert.Contains(t, formats, "yaml")
	assert.Contains(t, formats, "human")
}

func TestSupportedFormatsPure(t *testing.T) {
	// Get formats twice
	formats1 := SupportedFormats()
	formats2 := SupportedFormats()

	// They should be equal but not the same slice
	assert.Equal(t, formats1, formats2)

	// Modify one and verify the other is unchanged
	formats1[0] = "modified"
	assert.NotEqual(t, formats1[0], formats2[0])
}

func TestOutputFormatConstants(t *testing.T) {
	assert.Equal(t, OutputFormatJSON, OutputFormat("json"))
	assert.Equal(t, OutputFormatYAML, OutputFormat("yaml"))
	assert.Equal(t, OutputFormatHumanReadable, OutputFormat("human"))
}
