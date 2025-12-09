#!/bin/bash

# List of servers to extract tool registrations from
SERVERS=(
  "github-official"
  "gitmcp"
  "slack"
  "fetch"
  "duckduckgo"
  "brave"
  "context7"
  "dockerhub"
  "playwright"
  "wikipedia-mcp"
  "SQLite"
  "notion-remote"
  "rust-mcp-filesystem"
  "arxiv-mcp-server"
  "google-maps"
  "google-maps-comprehensive"
  "hugging-face"
  "linkedin-mcp-server"
  "desktop-commander"
  "openbnb-airbnb"
  "youtube_transcript"
  "time"
  "sequentialthinking"
  "semgrep"
  "resend"
  "papersearch"
  "openweather"
  "openapi-schema"
  "openapi"
  "node-code-sandbox"
  "minecraft-wiki"
  "microsoft-learn"
  "memory"
  "mcp-hackernews"
  "maven-tools-mcp"
  "markitdown"
  "gemini-api-docs"
  "filesystem"
  "everart"
  "elevenlabs"
  "stripe"
)

# Common configuration
CATALOG="$HOME/.docker/mcp/catalogs/docker-mcp.yaml"
CONFIG="./config.yaml"

# Loop through each server and extract tool registrations
for SERVER in "${SERVERS[@]}"; do
  OUTPUT=./tool-json/"${SERVER}.json"
  echo "Extracting tools from ${SERVER}..."

  go run main.go \
    -catalog "${CATALOG}" \
    -server "${SERVER}" \
    -config "${CONFIG}" \
    -output "${OUTPUT}"

  if [ $? -eq 0 ]; then
    echo "✓ Successfully extracted tools from ${SERVER} to ${OUTPUT}"
  else
    echo "✗ Failed to extract tools from ${SERVER}"
  fi
  echo ""
done

echo "Done! Extracted tool registrations from ${#SERVERS[@]} servers."
