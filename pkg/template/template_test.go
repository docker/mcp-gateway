package template

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestFindByID(t *testing.T) {
	tests := []struct {
		id       string
		wantNil  bool
		wantID   string
		wantName string
	}{
		{id: "ai-coding", wantNil: false, wantID: "ai-coding", wantName: "AI coding"},
		{id: "dev-workflow", wantNil: false, wantID: "dev-workflow", wantName: "Dev workflow"},
		{id: "terminal-control", wantNil: false, wantID: "terminal-control", wantName: "Terminal control"},
		{id: "nonexistent", wantNil: true},
		{id: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			tmpl := FindByID(tt.id)
			if tt.wantNil {
				assert.Nil(t, tmpl)
			} else {
				require.NotNil(t, tmpl)
				assert.Equal(t, tt.wantID, tmpl.ID)
				assert.Equal(t, tt.wantName, tmpl.Title)
				assert.NotEmpty(t, tmpl.Description)
				assert.NotEmpty(t, tmpl.ServerNames)
			}
		})
	}
}

func TestCatalogServerRef(t *testing.T) {
	tests := []struct {
		id      string
		wantRef string
	}{
		{
			id:      "ai-coding",
			wantRef: "catalog://mcp/docker-mcp-catalog/context7+sequentialthinking",
		},
		{
			id:      "dev-workflow",
			wantRef: "catalog://mcp/docker-mcp-catalog/github-official+atlassian-remote",
		},
		{
			id:      "terminal-control",
			wantRef: "catalog://mcp/docker-mcp-catalog/desktop-commander+filesystem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			tmpl := FindByID(tt.id)
			require.NotNil(t, tmpl)
			assert.Equal(t, tt.wantRef, tmpl.CatalogServerRef())
		})
	}
}

func TestTemplatesHaveUniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, tmpl := range Templates {
		assert.False(t, seen[tmpl.ID], "duplicate template ID: %s", tmpl.ID)
		seen[tmpl.ID] = true
	}
}

func TestTemplatesHaveRequiredFields(t *testing.T) {
	for _, tmpl := range Templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			assert.NotEmpty(t, tmpl.ID)
			assert.NotEmpty(t, tmpl.Title)
			assert.NotEmpty(t, tmpl.Description)
			assert.GreaterOrEqual(t, len(tmpl.ServerNames), 1)
			for _, name := range tmpl.ServerNames {
				assert.NotEmpty(t, name)
			}
		})
	}
}

// captureStdout captures stdout during function execution.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestListHumanReadable(t *testing.T) {
	output := captureStdout(func() {
		err := List(workingset.OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "ai-coding")
	assert.Contains(t, output, "dev-workflow")
	assert.Contains(t, output, "terminal-control")
	assert.Contains(t, output, "AI coding")
	assert.Contains(t, output, "context7")
}

func TestListJSON(t *testing.T) {
	output := captureStdout(func() {
		err := List(workingset.OutputFormatJSON)
		require.NoError(t, err)
	})

	var templates []Template
	err := json.Unmarshal([]byte(output), &templates)
	require.NoError(t, err)
	assert.Len(t, templates, len(Templates))
	assert.Equal(t, "ai-coding", templates[0].ID)
}

func TestListYAML(t *testing.T) {
	output := captureStdout(func() {
		err := List(workingset.OutputFormatYAML)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "ai-coding")
	assert.Contains(t, output, "context7")
}

func TestListUnsupportedFormat(t *testing.T) {
	err := List(workingset.OutputFormat("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}
