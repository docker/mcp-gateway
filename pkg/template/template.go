package template

import "strings"

// DefaultCatalogRef is the OCI reference for Docker's default MCP catalog.
const DefaultCatalogRef = "mcp/docker-mcp-catalog"

// Template defines a starter profile template that bundles a curated set of
// MCP servers from the Docker catalog.
type Template struct {
	ID          string   `json:"id" yaml:"id"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	ServerNames []string `json:"server_names" yaml:"server_names"`
}

// Templates is the list of built-in starter templates.
var Templates = []Template{
	{
		ID:          "ai-coding",
		Title:       "AI coding",
		Description: "Write code faster with Context7 for codebase awareness and Sequential Thinking for structured problem-solving.",
		ServerNames: []string{"context7", "sequentialthinking"},
	},
	{
		ID:          "dev-workflow",
		Title:       "Dev workflow",
		Description: "Automate your development cycle: open issues, write code, and update tickets with GitHub and Atlassian.",
		ServerNames: []string{"github-official", "atlassian-remote"},
	},
	{
		ID:          "terminal-control",
		Title:       "Terminal control",
		Description: "Run commands and scripts, manage files, and control your system directly from your AI client.",
		ServerNames: []string{"desktop-commander", "filesystem"},
	},
}

// FindByID returns the template with the given ID, or nil if not found.
func FindByID(id string) *Template {
	for i := range Templates {
		if Templates[i].ID == id {
			return &Templates[i]
		}
	}
	return nil
}

// CatalogServerRef returns the catalog:// URI for use with profile creation.
// For example: "catalog://mcp/docker-mcp-catalog/context7+sequentialthinking"
func (t *Template) CatalogServerRef() string {
	return "catalog://" + DefaultCatalogRef + "/" + strings.Join(t.ServerNames, "+")
}
