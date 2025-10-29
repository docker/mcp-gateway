# {{.ServerName}} MCP Server

A simple Model Context Protocol (MCP) server written in Go that provides a greeting tool.

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

## Tools

- **greet**: Says hi to a specified person
  - Input: `name` (string) - the person to greet
  - Output: A greeting message

## Development

To modify the server, edit `main.go` and rebuild the Docker image.
