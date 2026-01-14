# MCP Server Entry Specification

This document defines the specification for MCP server entries in the Docker MCP Gateway catalog system.

Server entries can be defined for an mcp server by writing a yaml file and using it as a CLI flag for profiles or catalogs via `--server file://my-server.yaml`.

**A note about legacy catalogs:** Legacy catalogs such as `.docker/mcp/catalogs/docker-mcp.yaml` or http://desktop.docker.com/mcp/catalog/v3/catalog.yaml use a similar schema for servers under the `registry` property. However, this spec is intended for defining server configurations for MCP Profiles and OCI Catalogs. Thus, it's expected that this spec will drift from what exists in legacy catalogs.


## Example Server Entry YAML

```yaml
name: duckduckgo
title: DuckDuckGo
type: server
image: mcp/duckduckgo@sha256:68eb20db6109f5c312a695fc5ec3386ad15d93ffb765a0b4eb1baf4328dec14f
description: A Model Context Protocol (MCP) server that provides web search capabilities through DuckDuckGo, with additional features for content fetching and parsing.
allowHosts:
  - html.duckduckgo.com:443
```

## Server Entry Structure

### Core Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **Yes** | Unique identifier for the server. Used for referencing and managing the server. |
| `type` | string | **Yes** | Server type. Must be one of: `server`, `remote`, or `poci`. |
| `title` | string | **Yes** | Human-readable display name for the server. |
| `description` | string | **Yes** | Brief description of the server's capabilities and purpose. |
| `icon` | string | No | URL to an icon/logo representing the server. |
| `readme` | string | No | URL to a README file with detailed documentation for the server. |

### Container Configuration (for type: "server")

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | Yes* | Docker image reference (can be SHA256 digest or tag). Required for `server` type. |
| `command` | []string | No | Command-line arguments to pass to the container. |
| `volumes` | []string | No | Volume mount specifications (format: `host:container` or `host:container:ro`). |
| `user` | string | No | User to run the container as (e.g., `1000:1000`). |
| `longLived` | boolean | No | Whether the server should remain running (true) or start on-demand (false). Default: false. |

### Remote Configuration (for type: "remote")

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `remote` | Remote | Yes* | Remote server configuration. Required for `remote` type. |
| `sseEndpoint` | string | No | **Deprecated**: Legacy SSE endpoint URL. Use `remote` instead. |

**Remote Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | URL endpoint for the remote MCP server. |
| `transport_type` | string | No | Transport protocol type (e.g., `sse` for Server-Sent Events). |
| `headers` | map[string]string | No | Custom HTTP headers to send with requests. |

### Authentication & Secrets

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secrets` | []Secret | No | API keys and secrets required by the server. |
| `oauth` | OAuth | No | OAuth configuration for authentication flows. |

**Secret Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Name of the secret. Must be prefixed by unique name of the server (e.g., `brave.api_key`). |
| `env` | string | Yes | Environment variable name to inject the secret as (e.g., `BRAVE_API_KEY`). |
| `example` | string | Yes | An example value for the key (e.g. `your_api_key`) |

**OAuth Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `providers` | []OAuthProvider | Yes | List of OAuth providers supported. |
| `scopes` | []string | No | OAuth scopes to request. |

**OAuthProvider Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | OAuth provider identifier (e.g., `github`, `google`). |
| `secret` | string | Yes | Reference to stored OAuth credentials. Must be prefixed by unique name of the server (e.g., `asana.personal_access_token`) |
| `env` | string | Yes | Environment variable to inject OAuth token as. |

### Environment Variables

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `env` | []Env | No | Static environment variables to set in the container. |

**Env Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Environment variable name. (e.g. `ASTRA_DB_API_ENDPOINT`) |
| `value` | string | Yes | Environment variable value. Can reference config fields using template syntax (e.g., `{{astra-db.endpoint}}`). |

### Network & Security

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `disableNetwork` | boolean | No | If true, disables all network access for the container. Default: false. |
| `allowHosts` | []string | No | Whitelist of hosts/domains the server is allowed to access (e.g., `["api.github.com:443", "github.com:443"]`). |

### Tools Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tools` | []Tool | No | Array of tools provided by this server. Used for discovery and documentation. |

**Tool Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Tool identifier (unique within the server). |
| `description` | string | No | Human-readable description of what the tool does. |
| `arguments` | []ToolArgument | No | Tool argument definitions (only set for OCI catalogs, not legacy catalogs). |
| `annotations` | ToolAnnotations | No | Tool annotations with hints about behavior (only set for OCI catalogs). |

**ToolArgument Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Argument name. |
| `type` | string | No | JSON Schema type (`string`, `number`, `boolean`, `array`, etc.). |
| `desc` | string | No | Description of the argument. |
| `items` | Items | No | For array types, defines the item schema. |
| `optional` | boolean | No | Whether the argument is optional. Default: false. |

**ToolAnnotations Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | No | Human-readable title for the tool. |
| `readOnlyHint` | boolean | No | Hint that the tool only reads data and does not modify state. |
| `destructiveHint` | boolean | No | Hint that the tool may perform destructive operations. |
| `idempotentHint` | boolean | No | Hint that the tool is idempotent (repeated calls have the same effect). |
| `openWorldHint` | boolean | No | Hint that the tool interacts with external systems/world. |

### Configuration Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `config` | []ConfigObject | No | Array of configuration objects defining user-configurable settings for the server. Each config object represents a group of related settings that users must provide before using the server. |

**ConfigObject Structure:**

The config field accepts an array of configuration objects. Each object defines a set of user-configurable parameters that can be referenced in environment variables using template syntax (e.g., `{{server-name.property-name}}`).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier for this config object. Should match the server's unique name (e.g., `couchbase`, `desktop-commander`). |
| `description` | string | Yes | Human-readable description explaining what these settings configure. |
| `type` | string | Yes | Must be `object`. Defines this as a configuration object. |
| `properties` | map[string]PropertySchema | Yes | Map of property names to their schema definitions. Each key is a field name that users will configure. |
| `required` | []string | No | Array of property names that must be provided by the user. |
| `anyOf` | []RequirementSet | No | Alternative requirement sets - user must satisfy at least one set. |
| `oneOf` | []RequirementSet | No | Exclusive requirement sets - user must satisfy exactly one set. |

**PropertySchema Structure:**

Defines the schema for a single configurable property:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | JSON Schema type: `string`, `number`, `boolean`, `array`, or `object`. |
| `description` | string | No | User-facing description explaining what this property configures. |
| `items` | ItemsSchema | No | For `array` types, defines the schema of array elements. |
| `properties` | map[string]PropertySchema | No | For `object` types, defines nested properties. |
| `required` | []string | No | For `object` types, lists required nested properties. |
| `default` | any | No | Default value if user doesn't provide one. |
| `enum` | []any | No | Allowed values for this property. |
| `pattern` | string | No | Regular expression pattern for string validation. |
| `minimum` | number | No | Minimum value for numbers. |
| `maximum` | number | No | Maximum value for numbers. |

**ItemsSchema Structure:**

For array-type properties:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Type of array elements (e.g., `string`, `number`, `object`). |

**RequirementSet Structure:**

For `anyOf` or `oneOf` conditional requirements:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `required` | []string | Yes | List of property names that must be provided together. |

**Config Examples:**

**Example 1: Simple String Properties**
```yaml
config:
  - name: couchbase
    description: "Configure the connection to Couchbase"
    type: object
    properties:
      cb_connection_string:
        type: string
        description: "Connection string for the Couchbase cluster"
      cb_username:
        type: string
        description: "Username for the Couchbase cluster with access to the bucket"
      cb_bucket_name:
        type: string
        description: "Bucket in the Couchbase cluster to use for the MCP server"
    required:
      - cb_connection_string
      - cb_username
      - cb_bucket_name
```

**Example 2: Array Type Configuration**
```yaml
config:
  - name: desktop-commander
    description: "Configure filesystem access and network permissions"
    type: object
    properties:
      paths:
        type: array
        description: "List of directories that Desktop Commander can access"
        items:
          type: string
    required:
      - paths
```

**Example 3: Nested Object Properties**
```yaml
config:
  - name: atlassian
    description: "The MCP server is allowed to access these paths"
    type: object
    properties:
      confluence:
        type: object
        properties:
          url:
            type: string
            description: "Confluence instance URL"
          username:
            type: string
            description: "Confluence username"
      jira:
        type: object
        properties:
          url:
            type: string
            description: "Jira instance URL"
          username:
            type: string
            description: "Jira username"
    anyOf:
      - required: [confluence]
      - required: [jira]
```

**Referencing Config Values:**

Config values can be referenced in environment variables and command arguments using template syntax:

```yaml
name: astra-db
config:
  - name: astra-db
    description: "Configure Astra DB connection"
    type: object
    properties:
      endpoint:
        type: string
        description: "Astra DB API endpoint URL"
      token:
        type: string
        description: "Astra DB application token"
    required:
      - endpoint
      - token
env:
  - name: ASTRA_DB_API_ENDPOINT
    value: "{{astra-db.endpoint}}"
  - name: ASTRA_DB_APPLICATION_TOKEN
    value: "{{astra-db.token}}"
```

### Metadata

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `metadata` | Metadata | No | Additional metadata about the server (for display and categorization). |

**Metadata Object Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pulls` | integer | No | Number of Docker image pulls. |
| `stars` | integer | No | Number of stars/ratings. |
| `githubStars` | integer | No | Number of GitHub stars for the source repository. |
| `category` | string | No | Category classification (e.g., `Development`, `Cloud`, `AI`). |
| `tags` | []string | No | Tags for searchability and filtering. |
| `license` | string | No | Software license (e.g., `MIT`, `Apache-2.0`). |
| `owner` | string | No | Owner/maintainer identifier. |

## Server Types

### Type: "server"

A containerized MCP server that runs as a Docker container managed by the gateway.

**Required fields:**
- `name`
- `type: "server"`
- `image`

**Example:**
```yaml
name: github-official
type: server
title: GitHub Official
description: Official GitHub MCP server for repository management
image: ghcr.io/github/github-mcp-server@sha256:a1d43076a36638ee24520fd6e83c3905ae41bc9850179081df1de2ba3a7afae0
icon: https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png
secrets:
  - name: github.personal_access_token
    env: GITHUB_PERSONAL_ACCESS_TOKEN
    example: <YOUR_TOKEN>
    description: You can create a GitHub personal access token [on GitHub](https://github.com/settings/personal-access-tokens/new)
allowHosts:
  - api.github.com:443
  - github.com:443
  - raw.githubusercontent.com:443
```

### Type: "remote"

An MCP server hosted remotely and accessed via HTTP/SSE protocol.

**Required fields:**
- `name`
- `type: "remote"`
- `remote.url`

**Example:**
```yaml
name: cloudflare-docs
description: Access the latest documentation on Cloudflare products such as Workers, Pages, R2, D1, KV.
title: Cloudflare Docs
type: remote
remote:
  transport_type: sse
  url: https://docs.mcp.cloudflare.com/sse
icon: https://www.cloudflare.com/favicon.ico
```

## Best Practices

1. **Naming**: Use lowercase, hyphen-separated names (e.g., `github-official`, `aws-core-mcp-server`)
2. **Images**: Prefer SHA256 digests for production, tags for development. Make sure images exist on a repository that can be accessed by those who can access the server entry (e.g. via code commit or in a private catalog)
3. **Security**:
   - Use `allowHosts` to restrict network access
   - Use `disableNetwork: true` for tools that don't need network
   - Always use secrets for credentials, never hardcode in `env`

