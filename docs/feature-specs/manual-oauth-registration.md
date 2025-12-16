# OAuth Providers in Docker CE Mode

**OAuth Flow Implementation for Standalone Environments**

## Overview

The MCP Gateway supports OAuth authentication for remote MCP servers in both Docker Desktop and Docker CE (standalone) modes. In Docker CE mode, the gateway handles all OAuth flows independently, including Dynamic Client Registration (DCR), token management, and automatic refresh.

This document explains the complete OAuth provider architecture when running in Docker CE mode.

## Mode Detection

Docker CE mode is detected via `pkg/oauth/mode.go:19`. The system uses CE mode when:
- Running inside a container
- On Linux when Docker Desktop is not detected
- Environment variable `DOCKER_MCP_USE_CE=true` is set

CE mode is essentially the inverse of Docker Desktop mode - when Docker Desktop isn't available, the MCP Gateway handles OAuth flows standalone.

## Key Differences from Docker Desktop Mode

| Aspect | Docker Desktop Mode | Docker CE Mode |
|--------|---------------------|----------------|
| OAuth Registration | Desktop app manages DCR | Gateway performs DCR |
| Auth UI | Unified Desktop UI | CLI + browser flow |
| Token Storage | Desktop backend | Docker credential helpers |
| Token Refresh | Desktop API | Gateway refresh loop |
| Callback Handling | Desktop proxy | Local callback server + mcp-oauth proxy |

## OAuth Flow Architecture (CE Mode)

### 1. Dynamic Client Registration (DCR)

When you run `docker mcp oauth authorize <server>`, the system first ensures a DCR client exists via `pkg/oauth/dcr/manager.go:43`:

```
Manager.PerformDiscoveryAndRegistration():
  ↓
1. OAuth Discovery (RFC 9728, RFC 8414)
   - Fetches /.well-known/oauth-authorization-server metadata from server
   - Discovers authorization endpoints, token endpoints, supported scopes
  ↓
2. Dynamic Client Registration (RFC 7591)
   - Registers a new OAuth client with the provider
   - Uses redirect URI: https://mcp.docker.com/oauth/callback
   - Receives client_id (and optionally client_secret for confidential clients)
  ↓
3. Store DCR Client
   - Saves to Docker credential helper with key: https://{serverName}.mcp-dcr
   - Stores: clientID, endpoints, scopes, provider name
```

**Key Files:**
- `pkg/oauth/dcr/manager.go:43` - DCR orchestration
- `pkg/oauth/dcr/credentials.go:57` - Credential storage
- Uses `github.com/docker/mcp-gateway-oauth-helpers` for RFC compliance

### 2. Authorization Flow

After DCR, the authorization flow begins in `cmd/docker-mcp/oauth/auth.go:42`:

```
authorizeCEMode():
  ↓
1. Start Local Callback Server (pkg/oauth/callback_server.go)
   - Binds to localhost:5000 (or MCP_GATEWAY_OAUTH_PORT)
   - Provides endpoint: http://localhost:5000/callback
  ↓
2. Build Authorization URL (pkg/oauth/manager.go:60)
   - Generates PKCE verifier (RFC 7636) for security
   - Creates state parameter: "mcp-gateway:5000:UUID"
   - Adds resource parameter (RFC 8707) for token audience binding
   - Constructs URL: {auth_endpoint}?client_id=...&redirect_uri=...&state=...&code_challenge=...
  ↓
3. User Opens Browser
   - User authenticates with OAuth provider
   - Provider redirects to: https://mcp.docker.com/oauth/callback?code=...&state=...
   - mcp-oauth proxy (Docker infrastructure) routes to localhost:5000 based on state
  ↓
4. Callback Received (pkg/oauth/callback_server.go:111)
   - Local server receives authorization code and state
   - Displays success page to user
  ↓
5. Token Exchange (pkg/oauth/manager.go:126)
   - Validates state and retrieves PKCE verifier
   - Exchanges authorization code for access token + refresh token
   - Uses PKCE verifier for security
  ↓
6. Token Storage (pkg/oauth/token_store.go)
   - Stores tokens in Docker credential helper
   - Key format: {auth_endpoint}/{provider_name}
   - Value: base64-encoded JSON with access_token, refresh_token, expiry
```

**Key Components:**
- `cmd/docker-mcp/oauth/auth.go:42` - Authorization orchestration
- `pkg/oauth/callback_server.go` - Local HTTP server for OAuth callbacks
- `pkg/oauth/manager.go` - OAuth flow management
- `pkg/oauth/token_store.go` - Token persistence

### 3. Credential Storage

All credentials use Docker credential helpers (`pkg/oauth/credhelper.go:201`):

#### DCR Clients

Stored in credential helper with:
- **Key:** `https://{serverName}.mcp-dcr`
- **Username:** `dcr_client`
- **Secret:** Base64-encoded JSON containing:
  ```json
  {
    "serverName": "my-server",
    "providerName": "my-server",
    "clientId": "...",
    "clientSecret": "...",
    "authorizationEndpoint": "https://...",
    "tokenEndpoint": "https://...",
    "resourceUrl": "https://...",
    "scopesSupported": ["read", "write"],
    "requiredScopes": ["read"],
    "registeredAt": "2025-12-16T12:00:00Z"
  }
  ```

#### OAuth Tokens

Stored in credential helper with:
- **Key:** `{authorizationEndpoint}/{providerName}`
- **Username:** `oauth_token`
- **Secret:** Base64-encoded JSON containing:
  ```json
  {
    "access_token": "eyJ...",
    "token_type": "Bearer",
    "refresh_token": "...",
    "expiry": "2025-12-16T12:00:00Z"
  }
  ```

#### Credential Helper Types

The system uses different credential helper instances:
- **Read-only** (`NewOAuthCredentialHelper` at line 31): For reading tokens during gateway operations
- **Read-write** (`NewReadWriteCredentialHelper` at line 217): For storing DCR clients and tokens

### 4. Automatic Token Refresh

The gateway runs a background refresh loop for each OAuth-enabled server in `pkg/oauth/provider.go:93`:

```
Provider.Run():
  ↓
Loop:
  1. Check token status (GetTokenStatus at credhelper.go:110)
     - Parses token expiry from credential helper
     - Token needs refresh if expiry <= 10 seconds
  ↓
  2. If needs refresh (provider.go:145):
     CE Mode:
       - Call refreshTokenCE() (line 208)
       - Retrieves DCR client and current token
       - Uses oauth2.Config.TokenSource() for automatic refresh
       - Saves refreshed token back to credential helper

     Desktop Mode:
       - Calls Desktop API to trigger refresh
  ↓
  3. Wait with exponential backoff
     - First attempt: 30s
     - Subsequent attempts: 1min, 2min, 4min, 8min...
     - Max 7 retry attempts (maxRefreshRetries at line 78)
  ↓
  4. Listen for events
     - SSE events from mcp-oauth proxy
     - EventLoginSuccess / EventTokenRefresh reset retry count
```

**Refresh Logic:**
- Uses `golang.org/x/oauth2` library's built-in refresh mechanism
- Automatically handles refresh token exchange
- Updates token expiry after successful refresh
- Exponential backoff prevents hammering provider APIs

**Key Files:**
- `pkg/oauth/provider.go:93` - Background refresh loop
- `pkg/oauth/provider.go:208` - CE mode token refresh
- `pkg/oauth/credhelper.go:110` - Token status check

### 5. Token Retrieval During Runtime

When MCP servers need tokens (`pkg/oauth/credhelper.go:49`):

```
CredentialHelper.GetOAuthToken():
  ↓
1. Determine credential key
   CE Mode: Read DCR client from credential helper
   Desktop Mode: Call Desktop API for DCR client
  ↓
2. Construct key: {authEndpoint}/{providerName}
  ↓
3. Retrieve from credential helper
  ↓
4. Decode base64 JSON
  ↓
5. Extract access_token field
  ↓
6. Return to MCP server as environment variable or header
```

## Manual Registration

For servers that don't support DCR, you can manually register pre-configured OAuth clients:

```bash
# Register with client ID and secret (confidential client)
docker mcp oauth register my-server \
  --client-id "abc123" \
  --client-secret "secret456" \
  --auth-endpoint "https://provider.com/oauth/authorize" \
  --token-endpoint "https://provider.com/oauth/token" \
  --scopes "read,write"

# Register public client (no secret)
docker mcp oauth register my-server \
  --client-id "public-client-id" \
  --auth-endpoint "https://provider.com/oauth/authorize" \
  --token-endpoint "https://provider.com/oauth/token"
```

This command stores a pre-configured DCR client, skipping the discovery/registration steps. After registration, authorize normally:

```bash
docker mcp oauth authorize my-server
```

**Implementation:**
- `cmd/docker-mcp/commands/oauth.go:66` - Command definition
- `cmd/docker-mcp/oauth/register.go` - Registration handler

## Security Features

1. **PKCE** (Proof Key for Code Exchange, RFC 7636)
   - All flows use S256 challenge method
   - Generated in `pkg/oauth/provider.go:59-63`
   - Protects against authorization code interception

2. **State Parameter**
   - Prevents CSRF attacks
   - Format: `mcp-gateway:{port}:{uuid}`
   - Includes routing info for mcp-oauth proxy
   - Managed by `pkg/oauth/state.go`

3. **Credential Helpers**
   - Tokens stored in OS-native secure storage
   - macOS: Keychain
   - Linux: Secret Service / pass
   - Windows: Credential Manager
   - Uses `github.com/docker/docker-credential-helpers`

4. **Token Audience Binding** (RFC 8707)
   - Resource parameter ties tokens to specific servers
   - Prevents token reuse across services
   - Set in `pkg/oauth/manager.go:115-117`

5. **Container Isolation**
   - MCP servers run in containers
   - Can't directly access credential storage
   - Gateway injects tokens at runtime

## Configuration

### Environment Variables

- `MCP_GATEWAY_OAUTH_PORT`: Custom OAuth callback port (default: 5000)
  - Used when default port is unavailable
  - Must be in range 1024-65535
  - Configured in `pkg/oauth/callback_server.go:36`

- `DOCKER_MCP_USE_CE`: Force CE mode even on Docker Desktop
  - Useful for testing CE flows on Desktop
  - Set to `true` to enable

### Credential Helper Configuration

The gateway automatically detects credential helpers using:
1. Docker config file (`~/.docker/config.json`)
2. Platform defaults (osxkeychain, secretservice, wincred)

## CLI Commands

### List OAuth Apps

```bash
docker mcp oauth ls [--json]
```

Lists all registered OAuth applications and their authorization status.

### Authorize an App

```bash
docker mcp oauth authorize <app> [--scopes "scope1 scope2"]
```

Initiates OAuth flow for an MCP server. In CE mode:
1. Performs DCR if needed
2. Starts local callback server
3. Opens browser for authentication
4. Waits for callback and exchanges code for token

### Revoke Authorization

```bash
docker mcp oauth revoke <app>
```

Removes stored tokens for an app. Does not revoke with provider.

### Manual Registration

```bash
docker mcp oauth register <server-name> \
  --client-id <id> \
  --client-secret <secret> \
  --auth-endpoint <url> \
  --token-endpoint <url> \
  --scopes <scopes>
```

Registers pre-configured OAuth credentials for servers without DCR support.

**Command Implementation:**
- `cmd/docker-mcp/commands/oauth.go` - Command definitions
- `cmd/docker-mcp/oauth/` - Command handlers

## File Reference

### Core OAuth Components

| File | Purpose | Key Functions |
|------|---------|---------------|
| `pkg/oauth/mode.go` | Mode detection | `IsCEMode()` - Determines if running in CE mode |
| `pkg/oauth/provider.go` | Provider lifecycle | `Run()` - Background refresh loop<br>`refreshTokenCE()` - CE mode token refresh |
| `pkg/oauth/manager.go` | OAuth orchestration | `BuildAuthorizationURL()` - Generate auth URLs<br>`ExchangeCode()` - Token exchange |
| `pkg/oauth/credhelper.go` | Credential access | `GetOAuthToken()` - Retrieve tokens<br>`GetTokenStatus()` - Check token validity |

### DCR Components

| File | Purpose | Key Functions |
|------|---------|---------------|
| `pkg/oauth/dcr/manager.go` | DCR orchestration | `PerformDiscoveryAndRegistration()` - DCR flow |
| `pkg/oauth/dcr/credentials.go` | DCR storage | `SaveClient()` - Store DCR client<br>`RetrieveClient()` - Load DCR client |

### Command Handlers

| File | Purpose | Key Functions |
|------|---------|---------------|
| `cmd/docker-mcp/oauth/auth.go` | Authorization | `authorizeCEMode()` - CE mode auth flow |
| `cmd/docker-mcp/oauth/ls.go` | List apps | `Ls()` - List OAuth apps |
| `cmd/docker-mcp/oauth/revoke.go` | Revocation | `Revoke()` - Remove tokens |
| `cmd/docker-mcp/oauth/register.go` | Manual registration | `Register()` - Manual client registration |

### Supporting Components

| File | Purpose | Key Functions |
|------|---------|---------------|
| `pkg/oauth/callback_server.go` | HTTP callback server | `Start()` - Run callback server<br>`Wait()` - Wait for callback |
| `pkg/oauth/token_store.go` | Token persistence | `Save()` - Store tokens<br>`Retrieve()` - Load tokens |
| `pkg/oauth/state.go` | State management | `Generate()` - Create state<br>`Validate()` - Verify state |

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        MCP Gateway                           │
│                                                              │
│  ┌────────────────┐      ┌─────────────────┐               │
│  │ OAuth Manager  │◄────►│  DCR Manager    │               │
│  │ (manager.go)   │      │ (dcr/manager.go)│               │
│  └───────┬────────┘      └────────┬────────┘               │
│          │                        │                         │
│          ▼                        ▼                         │
│  ┌────────────────┐      ┌─────────────────┐               │
│  │ Token Store    │      │ DCR Credentials │               │
│  │(token_store.go)│      │(dcr/creds.go)   │               │
│  └───────┬────────┘      └────────┬────────┘               │
│          │                        │                         │
│          └───────┬────────────────┘                         │
│                  ▼                                          │
│          ┌──────────────────┐                               │
│          │ Credential Helper│                               │
│          │ (credhelper.go)  │                               │
│          └─────────┬────────┘                               │
└────────────────────┼──────────────────────────────────────┘
                     │
                     ▼
        ┌─────────────────────────┐
        │  Docker Credential      │
        │  Helper                 │
        │  (osxkeychain/          │
        │   secretservice/        │
        │   wincred)              │
        └─────────────────────────┘
                     │
                     ▼
        ┌─────────────────────────┐
        │  OS Secure Storage      │
        │  (Keychain/SecretSvc/   │
        │   CredManager)          │
        └─────────────────────────┘

External Components:

┌──────────────────┐         ┌──────────────────┐
│ OAuth Provider   │◄───────►│ mcp-oauth Proxy  │
│ (e.g., Notion)   │         │ (Docker infra)   │
└──────────────────┘         └─────────┬────────┘
                                       │
                                       ▼
                            ┌──────────────────┐
                            │ Callback Server  │
                            │ (localhost:5000) │
                            └──────────────────┘
```

## Example Flow

### Full Authorization Flow

```bash
# 1. User initiates authorization
$ docker mcp oauth authorize notion-remote

# Gateway performs:
Starting OAuth authorization for notion-remote...

# 2. DCR (if needed)
Checking DCR registration...
- No DCR client found for notion-remote, performing registration...
- Starting OAuth discovery for: notion-remote at: https://notion.example.com
- Discovery successful for: notion-remote
- Registration successful for: notion-remote, clientID: abc123
- Stored DCR client for notion-remote

# 3. Start callback server
OAuth callback server bound to localhost:5000

# 4. Generate authorization URL
Generating authorization URL...
- Generated authorization URL for notion-remote with PKCE
- State format for proxy: mcp-gateway:5000:UUID

# 5. User authenticates
Please visit this URL to authorize:

  https://provider.com/oauth/authorize?client_id=abc123&redirect_uri=...

Waiting for authorization callback on http://localhost:5000/callback...

# 6. Provider redirects → mcp-oauth proxy → localhost:5000
- Received OAuth callback with code and state

# 7. Token exchange
Exchanging authorization code for access token...
- Exchanging authorization code for notion-remote
- Token exchanged for notion-remote (access: true, refresh: true)

# 8. Success
Authorization successful! Token stored securely.
You can now use: docker mcp server start notion-remote
```

### Token Refresh Loop

```bash
# Gateway starts provider loop for each OAuth server
$ docker mcp gateway run

# Background loop output:
- Started OAuth provider loop for notion-remote
- Token valid for notion-remote, next check in 3590s
# ... time passes ...
- Triggering token refresh for notion-remote, attempt 1/7, waiting 30s
- Successfully refreshed token for notion-remote
- Token valid for notion-remote, next check in 3590s
```

## Troubleshooting

### Port Already in Use

If port 5000 is already in use:

```bash
# Option 1: Use custom port
export MCP_GATEWAY_OAUTH_PORT=5001
docker mcp oauth authorize <app>

# Option 2: Find what's using the port
lsof -i :5000

# Option 3: Kill the conflicting process
kill $(lsof -t -i :5000)
```

### No Credential Helper Found

If credential helper is missing:

```bash
# Install credential helper
# macOS:
brew install docker-credential-helper

# Linux:
apt-get install pass  # or install gnome-keyring

# Configure Docker to use it
cat > ~/.docker/config.json <<EOF
{
  "credsStore": "osxkeychain"  # or "secretservice" on Linux
}
EOF
```

### Token Refresh Failures

If tokens aren't refreshing:

```bash
# Check token status
docker mcp oauth ls --json

# Re-authorize if refresh token is invalid
docker mcp oauth revoke <app>
docker mcp oauth authorize <app>
```

### CE Mode Not Detected

Force CE mode explicitly:

```bash
export DOCKER_MCP_USE_CE=true
docker mcp oauth authorize <app>
```

## Standards Compliance

The implementation follows these RFCs:

- **RFC 6749**: OAuth 2.0 Authorization Framework
- **RFC 7591**: OAuth 2.0 Dynamic Client Registration Protocol
- **RFC 7636**: Proof Key for Code Exchange (PKCE)
- **RFC 8414**: OAuth 2.0 Authorization Server Metadata
- **RFC 8707**: Resource Indicators for OAuth 2.0
- **RFC 9728**: OAuth 2.0 Device Authorization Grant (discovery)

External library used: `github.com/docker/mcp-gateway-oauth-helpers`

## Future Enhancements

Potential improvements for CE mode OAuth:

1. **Token Revocation**: Call provider revocation endpoint on `oauth revoke`
2. **Multi-tenant Support**: Support multiple OAuth tenants per server
3. **Refresh Token Rotation**: Implement refresh token rotation for enhanced security
4. **Browser Detection**: Auto-open browser on authorization
5. **OAuth Cache**: Cache discovery metadata to reduce provider calls
6. **Credential Migration**: Migrate credentials between helpers
