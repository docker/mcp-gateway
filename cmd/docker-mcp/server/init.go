package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	mainGoTemplate = `package main

import (
	"context"
	_ "embed"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed ui.html
var uiHTML string

func main() {
	// Create a server with a single tool that says "Hi".
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)

	// Register the UI HTML as a resource for ChatGPT Apps
	server.AddResource(&mcp.Resource{
		URI:         "ui://greeter/widget.html",
		Name:        "Greeter Widget",
		Description: "Interactive UI for the greeter tool",
		MIMEType:    "text/html+skybridge",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "ui://greeter/widget.html",
					MIMEType: "text/html+skybridge",
					Text:     uiHTML,
				},
			},
		}, nil
	})

	// Using the generic AddTool automatically populates the input and output
	// schema of the tool.
	//
	// The schema considers 'json' and 'jsonschema' struct tags to get argument
	// names and descriptions.
	type args struct {
		Name         string ` + "`json:\"name\" jsonschema:\"the person to greet\"`" + `
		GreetingType string ` + "`json:\"greetingType,omitempty\" jsonschema:\"type of greeting (Hi, Hey, or Hello)\"`" + `
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "greet someone with a customizable greeting",
		Meta: mcp.Meta{
			"openai/outputTemplate":          "ui://greeter/widget.html",
			"openai/toolInvocation/invoking": "Greeting...",
			"openai/widgetAccessible":        true,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
		// Default to "Hi" if not specified
		greetingType := args.GreetingType
		if greetingType == "" {
			greetingType = "Hi"
		}
		greeting := greetingType + " " + args.Name + "!"

		// Return structured data that the UI can consume
		structuredData := map[string]interface{}{
			"greeting":     greeting,
			"name":         args.Name,
			"greetingType": greetingType,
		}

		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: greeting},
			},
		}

		// Add metadata linking to the UI resource
		// Note: ChatGPT Apps integration requires protocol-level support
		result.Meta.SetMeta(map[string]interface{}{
			"outputTemplate":    "ui://greeter/widget.html",
			"structuredContent": structuredData,
		})

		return result, structuredData, nil
	})

	// server.Run runs the server on the given transport.
	//
	// In this case, the server communicates over stdin/stdout.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
`

	uiHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            border-radius: 12px;
            padding: 40px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            max-width: 400px;
            width: 100%;
        }
        h1 {
            color: #333;
            margin-bottom: 24px;
            text-align: center;
            font-size: 28px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 8px;
            color: #555;
            font-weight: 500;
        }
        input[type="text"] {
            width: 100%;
            padding: 12px 16px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 16px;
            transition: border-color 0.3s;
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #667eea;
        }
        select {
            width: 100%;
            padding: 12px 16px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 16px;
            background-color: white;
            cursor: pointer;
            transition: border-color 0.3s;
        }
        select:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(102, 126, 234, 0.4);
        }
        button:active {
            transform: translateY(0);
        }
        button:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .response {
            margin-top: 24px;
            padding: 16px;
            background: #f5f5f5;
            border-radius: 8px;
            border-left: 4px solid #667eea;
            display: none;
        }
        .response.show {
            display: block;
            animation: fadeIn 0.3s;
        }
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(-10px); }
            to { opacity: 1; transform: translateY(0); }
        }
        .response-text {
            color: #333;
            font-size: 18px;
            font-weight: 500;
        }
        .error {
            border-left-color: #ef4444;
            background: #fee;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>👋 Greeter</h1>
        <div class="form-group">
            <label for="greetingType">Choose a greeting:</label>
            <select id="greetingType">
                <option value="Hi">Hi</option>
                <option value="Hey">Hey</option>
                <option value="Hello">Hello</option>
            </select>
        </div>
        <div class="form-group">
            <label for="nameInput">What's your name?</label>
            <input type="text" id="nameInput" placeholder="Enter your name" autofocus>
        </div>
        <button id="greetBtn">Greet Me</button>
        <div id="response" class="response">
            <div class="response-text" id="responseText"></div>
        </div>
    </div>
    <script>
        const greetingType = document.getElementById('greetingType');
        const nameInput = document.getElementById('nameInput');
        const greetBtn = document.getElementById('greetBtn');
        const response = document.getElementById('response');
        const responseText = document.getElementById('responseText');

        function resetButton() {
            greetBtn.disabled = false;
            greetBtn.textContent = 'Greet Me';
        }

        greetBtn.addEventListener('click', handleGreet);
        nameInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') handleGreet();
        });

        async function handleGreet() {
            const name = nameInput.value.trim();
            if (!name) {
                showResponse('Please enter your name', true);
                return;
            }

            const greeting = greetingType.value;
            greetBtn.disabled = true;
            greetBtn.textContent = 'Greeting...';

            try {
                // Call the MCP tool using the ChatGPT Apps SDK
                const result = await window.openai.callTool('greet', {
                    name: name,
                    greetingType: greeting
                });

                // Display the result from structured content
                if (result && result.structuredContent && result.structuredContent.greeting) {
                    showResponse(result.structuredContent.greeting, false);
                } else if (result && result.content && result.content[0]) {
                    showResponse(result.content[0].text, false);
                } else {
                    showResponse('Received response but no greeting found', true);
                }
            } catch (error) {
                showResponse('Error: ' + error.message, true);
            } finally {
                resetButton();
            }
        }

        function showResponse(text, isError) {
            responseText.textContent = text;
            response.className = 'response show' + (isError ? ' error' : '');
        }
    </script>
</body>
</html>
`

	dockerfileTemplate = `FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (required for go mod download)
RUN apk add --no-cache git

# Copy source code
COPY . .

# Download dependencies and build the application
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o /mcp-server .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /mcp-server .

# Run the server
ENTRYPOINT ["./mcp-server"]
`

	composeTemplate = `services:
  gateway:
    image: docker/mcp-gateway
    command:
      - --servers=greeter
      - --catalog=/mcp/catalog.yaml
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./catalog.yaml:/mcp/catalog.yaml
    stdin_open: true
    tty: true
`

	catalogTemplate = `registry:
  greeter:
    description: A simple MCP server that greets users
    title: Greeter
    type: server
    image: greeter:latest
`

	goModTemplate = `module greeter

go 1.24

require github.com/modelcontextprotocol/go-sdk v1.0.0
`

	goSumTemplate = ``

	readmeTemplate = `# Greeter MCP Server

A simple Model Context Protocol (MCP) server written in Go that provides a greeting tool.

## Building

Build the Docker image:

` + "```bash" + `
docker build -t greeter:latest .
` + "```" + `

## Running with Docker Compose

Start the gateway with the greeter server:

` + "```bash" + `
docker compose up
` + "```" + `

The gateway will start and connect to the greeter server. You can then interact with it through MCP clients.

## Tools

- **greet**: Says hi to a specified person
  - Input: ` + "`name`" + ` (string) - the person to greet
  - Output: A greeting message

## Development

To modify the server, edit ` + "`main.go`" + ` and rebuild the Docker image.
`
)

// Init initializes a new MCP server project in the specified directory
func Init(ctx context.Context, dir string, language string) error {
	if language != "go" {
		return fmt.Errorf("unsupported language: %s (currently only 'go' is supported)", language)
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

	// Generate templated content with server name
	files := map[string]string{
		"main.go":       mainGoTemplate,
		"Dockerfile":    dockerfileTemplate,
		"compose.yaml":  generateComposeTemplate(serverName),
		"catalog.yaml":  generateCatalogTemplate(serverName),
		"go.mod":        generateGoModTemplate(serverName),
		"README.md":     generateReadmeTemplate(serverName),
		"ui.html":       generateUIHTML(serverName),
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	return nil
}

func generateComposeTemplate(serverName string) string {
	return fmt.Sprintf(`services:
  gateway:
    image: docker/mcp-gateway
    command:
      - --servers=%s
      - --catalog=/mcp/catalog.yaml
      - --transport=streaming
      - --port=8811
    environment:
      - DOCKER_MCP_IN_CONTAINER=1
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./catalog.yaml:/mcp/catalog.yaml
    ports:
      - "8811:8811"
`, serverName)
}

func generateCatalogTemplate(serverName string) string {
	return fmt.Sprintf(`registry:
  %s:
    description: A simple MCP server that greets users
    title: %s
    type: server
    image: %s:latest
`, serverName, serverName, serverName)
}

func generateGoModTemplate(serverName string) string {
	return fmt.Sprintf(`module %s

go 1.24

require github.com/modelcontextprotocol/go-sdk v1.0.0
`, serverName)
}

func generateReadmeTemplate(serverName string) string {
	return fmt.Sprintf(`# %s MCP Server

A simple Model Context Protocol (MCP) server written in Go that provides a greeting tool with a ChatGPT App UI.

## Features

- **MCP Tool**: A `+"`greet`"+` tool that says hi to any person
- **ChatGPT App UI**: Interactive web interface with a text input and button
- **Structured Data**: Returns both text content and structured data for the UI

## Building

Build the Docker image:

`+"```bash"+`
docker build -t %s:latest .
`+"```"+`

## Running with Docker Compose

Start the gateway with the %s server in streaming mode:

`+"```bash"+`
docker compose up
`+"```"+`

The gateway will start in streaming mode on port 8811 and connect to the %s server. You can then interact with it through MCP clients using HTTP streaming at http://localhost:8811.

## ChatGPT App UI

This server includes a beautiful interactive UI (`+"`ui.html`"+`) that demonstrates what a ChatGPT App interface could look like. The UI features:

- A text input field for entering a name
- A "Say Hi" button to trigger the greeting
- Animated response display with the greeting message
- Modern, responsive design with gradient styling

The UI is registered as an MCP resource at `+"`ui://greeter/widget.html`"+` with the `+"`text/html+skybridge`"+` MIME type. The `+"`greet`"+` tool includes metadata (`+"`_meta[\"openai/outputTemplate\"]`"+`) that links it to the UI, so when you call the tool from ChatGPT, the UI will render inline with the response.

## Tools

- **greet**: Says hi to a specified person
  - Input: `+"`name`"+` (string) - the person to greet
  - Output: A greeting message
  - UI: Interactive widget with form and response display

## Development

To modify the server:
- Edit `+"`main.go`"+` to change the tool logic or UI HTML
- Rebuild the Docker image with `+"`docker build -t %s:latest .`"+`
- Restart the gateway with `+"`docker compose up`"+`
`, serverName, serverName, serverName, serverName, serverName)
}

func generateUIHTML(serverName string) string {
	return uiHTMLTemplate
}
