# {{.ServerName}} MCP Server

A simple Model Context Protocol (MCP) server written in Go that provides a greeting tool with a ChatGPT App UI.

## Features

- **MCP Tool**: A `greet` tool that says hi to any person
- **ChatGPT App UI**: Interactive web interface with a dropdown and text input
- **Structured Data**: Returns both text content and structured data for the UI

## Building

Build the Docker image:

```bash
docker build -t {{.ServerName}}:latest .
```

## Running with Docker Compose

Start the gateway with the {{.ServerName}} server in streaming mode:

```bash
docker compose up
```

The gateway will start in streaming mode on port 8811 and connect to the {{.ServerName}} server. You can then interact with it through MCP clients using HTTP streaming at http://localhost:8811.

## ChatGPT App UI Setup

To use the ChatGPT App UI with this server:

1. **Expose the gateway publicly** with ngrok:
   ```bash
   ngrok http 8811
   ```

2. **Open ChatGPT Settings** on ChatGPT.com (not the app), then click "Apps and Connectors"

3. **Enable Developer Mode** at the bottom of the settings pane

4. **Create a new connector**:
   - Click the "Create" button at the top right
   - Give the connector a name (e.g., "{{.ServerName}}")
   - Add your ngrok URL
   - Set "Authentication" to None

5. **Use the connector** in ChatGPT:
   - Ask ChatGPT: "call the greet tool from the {{.ServerName}} server"
   - The interactive UI will appear inline
   - Select a greeting type (Hi, Hey, or Hello) from the dropdown
   - Enter a name in the text field
   - Click "Greet Me" to make the tool call

The UI will call the MCP tool via `window.openai.callTool()` and display the greeting response with smooth animations.

## Tools

- **greet**: Says hi to a specified person
  - Input: `name` (string) - the person to greet
  - Input: `greetingType` (string, optional) - type of greeting (Hi, Hey, or Hello)
  - Output: A greeting message
  - UI: Interactive widget with dropdown, form, and response display

## Development

To modify the server:
- Edit `main.go` to change the tool logic
- Edit `ui.html` to customize the UI
- Rebuild the Docker image with `docker build -t {{.ServerName}}:latest .`
- Restart the gateway with `docker compose up`
