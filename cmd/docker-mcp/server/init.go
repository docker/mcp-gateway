package server

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/basic/main.go
var basicMainGoTemplate string

//go:embed templates/basic/Dockerfile
var basicDockerfileTemplate string

//go:embed templates/basic/compose.yaml
var basicComposeTemplate string

//go:embed templates/basic/catalog.yaml
var basicCatalogTemplate string

//go:embed templates/basic/go.mod.template
var basicGoModTemplate string

//go:embed templates/basic/README.md
var basicReadmeTemplate string

//go:embed templates/chatgpt-app-basic/main.go
var chatgptAppMainGoTemplate string

//go:embed templates/chatgpt-app-basic/Dockerfile
var chatgptAppDockerfileTemplate string

//go:embed templates/chatgpt-app-basic/compose.yaml
var chatgptAppComposeTemplate string

//go:embed templates/chatgpt-app-basic/catalog.yaml
var chatgptAppCatalogTemplate string

//go:embed templates/chatgpt-app-basic/go.mod.template
var chatgptAppGoModTemplate string

//go:embed templates/chatgpt-app-basic/README.md
var chatgptAppReadmeTemplate string

//go:embed templates/chatgpt-app-basic/ui.html
var chatgptAppUITemplate string

type templateData struct {
	ServerName string
}

type templateSet struct {
	mainGo     string
	dockerfile string
	compose    string
	catalog    string
	goMod      string
	readme     string
	uiHTML     string // optional, only for chatgpt-app-basic
}

func getTemplateSet(templateName string) (*templateSet, error) {
	switch templateName {
	case "basic":
		return &templateSet{
			mainGo:     basicMainGoTemplate,
			dockerfile: basicDockerfileTemplate,
			compose:    basicComposeTemplate,
			catalog:    basicCatalogTemplate,
			goMod:      basicGoModTemplate,
			readme:     basicReadmeTemplate,
		}, nil
	case "chatgpt-app-basic":
		return &templateSet{
			mainGo:     chatgptAppMainGoTemplate,
			dockerfile: chatgptAppDockerfileTemplate,
			compose:    chatgptAppComposeTemplate,
			catalog:    chatgptAppCatalogTemplate,
			goMod:      chatgptAppGoModTemplate,
			readme:     chatgptAppReadmeTemplate,
			uiHTML:     chatgptAppUITemplate,
		}, nil
	default:
		return nil, fmt.Errorf("unknown template: %s (available: basic, chatgpt-app-basic)", templateName)
	}
}

// Init initializes a new MCP server project in the specified directory
func Init(ctx context.Context, dir string, language string, templateName string) error {
	if language != "go" {
		return fmt.Errorf("unsupported language: %s (currently only 'go' is supported)", language)
	}

	// Get the template set
	templates, err := getTemplateSet(templateName)
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("directory %s is not empty", dir)
	}

	// Extract server name from directory path
	serverName := filepath.Base(dir)
	data := templateData{ServerName: serverName}

	// Generate files from templates
	files := map[string]string{
		"main.go":       templates.mainGo,
		"Dockerfile":    templates.dockerfile,
		"compose.yaml":  templates.compose,
		"catalog.yaml":  templates.catalog,
		"go.mod":        templates.goMod,
		"README.md":     templates.readme,
	}

	// Add ui.html for chatgpt-app-basic template
	if templates.uiHTML != "" {
		files["ui.html"] = templates.uiHTML
	}

	for filename, tmplContent := range files {
		// Parse and execute template
		tmpl, err := template.New(filename).Parse(tmplContent)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", filename, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("executing template %s: %w", filename, err)
		}

		// Write file
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	return nil
}
