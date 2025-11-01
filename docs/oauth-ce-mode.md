# OAuth DCR with Docker CE

Complete guide for using MCP Gateway OAuth with Docker Engine (CE Mode).

## Prerequisites

### 1. Credential Helper Installed

MCP Gateway requires [Docker credential helper](https://github.com/docker/docker-credential-helpers) configured to securely store OAuth tokens.


Download from releases:
https://github.com/docker/docker-credential-helpers/releases

### 2. Configure Docker to Use Credential Helper

Edit or create `~/.docker/config.json`:

**macOS:**
```json
{
  "credsStore": "osxkeychain"
}
```

**Linux (Desktop):**
```json
{
  "credsStore": "secretservice"
}
```

**Linux (Headless):**
```json
{
  "credsStore": "pass"
}
```

**Windows:**
```json
{
  "credsStore": "wincred"
}
```

**Verify configuration:**
```bash
# Helper should be in your PATH
which docker-credential-osxkeychain  # macOS
which docker-credential-secretservice  # Linux Desktop
which docker-credential-pass  # Linux Headless
where docker-credential-wincred.exe  # Windows
```

For detailed installation instructions, see:
https://github.com/docker/docker-credential-helpers

## Configuration

### Enable CE Mode

Set the environment variable to enable standalone OAuth:

```bash
export DOCKER_MCP_USE_CE=true
```
### Optional: Customize OAuth Callback Port

By default, MCP Gateway listens on `localhost:5000` for OAuth callbacks.

If port 5000 is already in use, set a custom port:

```bash
export MCP_GATEWAY_OAUTH_PORT=5001
```

Valid range: 1024-65535

**Error handling:**
- If port is in use, you'll see a clear error message with solutions
- Invalid port values (< 1024 or > 65535) fall back to default with warning

## Usage

### Step-by-Step: Authorizing an OAuth Server

This example uses Notion, but works with any OAuth-enabled MCP server.

#### Step 1: Enable the Server

```bash
docker mcp server enable notion-remote
```

#### Step 2: Authorize

```bash
docker mcp oauth authorize notion-remote
```
