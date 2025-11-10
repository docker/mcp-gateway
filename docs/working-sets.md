# Working Sets

Working sets allow you to organize and manage collections of MCP servers as a single unit. Unlike catalogs that serve as repositories of available servers, working sets represent specific configurations of servers you want to use together for different purposes or contexts.

## What are Working Sets?

A working set is a named collection of MCP servers that can be:
- Created and managed independently of catalogs
- Shared across teams via export/import or OCI registries
- Used to quickly switch between different server configurations

Working sets are decoupled from catalogs, meaning the servers in a working set can come from:
- **MCP Registry references**: HTTP(S) URLs pointing to servers in the Model Context Protocol registry
- **OCI image references**: Docker images with the `docker://` prefix

## Enabling Working Sets

Working sets are a feature that must be enabled first:

```bash
# Enable the working-sets feature
docker mcp feature enable working-sets

# Verify it's enabled
docker mcp feature list
```

Once enabled, you'll have access to:
- `docker mcp workingset` commands for managing working sets
- `--working-set` flag for `docker mcp gateway run`
- `-w` or `--working-set` flag for `docker mcp client connect`

## Working Set Commands

### Creating Working Sets

Create a new working set with a name and list of servers:

```bash
# Create a working set with OCI image references
docker mcp workingset create --name dev-tools \
  --server docker://mcp/github:latest \
  --server docker://mcp/slack:latest

# Create with MCP Registry references
docker mcp workingset create --name registry-servers \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

# Mix MCP Registry and OCI references
docker mcp workingset create --name mixed \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 \
  --server docker://mcp/github:latest

# Specify a custom ID (otherwise derived from name)
docker mcp workingset create --name "My Servers" --id my-servers \
  --server docker://mcp/github:latest
```

**Notes:**
- `--name` is required and serves as the human-readable name
- `--id` is optional; if not provided, it's generated from the name (lowercase, alphanumeric with hyphens)
- `--server` can be specified multiple times to add multiple servers
- Server references must be either:
  - `docker://` prefix for OCI images
  - `http://` or `https://` URLs for MCP Registry references

### Listing Working Sets

View all working sets in your system:

```bash
# List all working sets (human-readable format)
docker mcp workingset list

# List with aliases
docker mcp workingset ls

# List in JSON format
docker mcp workingset list --format json

# List in YAML format
docker mcp workingset list --format yaml
```

**Output formats:**
- `human` (default): Tabular format with ID and Name columns
- `json`: Complete JSON representation
- `yaml`: Complete YAML representation

### Showing Working Set Details

Display detailed information about a specific working set:

```bash
# Show a working set (human-readable)
docker mcp workingset show my-working-set

# Show in JSON format
docker mcp workingset show my-working-set --format json

# Show in YAML format
docker mcp workingset show my-working-set --format yaml
```

The output includes:
- Working set ID and name
- List of servers with their types and references
- Configuration for each server
- Secrets configuration
- Tools associated with each server

### Removing Working Sets

Delete a working set from your system:

```bash
# Remove a working set
docker mcp workingset remove my-working-set

# Using alias
docker mcp workingset rm my-working-set
```

**Note:** This only removes the working set definition, not the actual server images or registry entries.

### Configuring Working Set Servers

Manage configuration values for servers within a working set:

```bash
# Set a single configuration value
docker mcp workingset config my-working-set --set github.timeout=30

# Set multiple configuration values
docker mcp workingset config my-working-set \
  --set github.timeout=30 \
  --set github.maxRetries=3 \
  --set slack.channel=general

# Get a specific configuration value
docker mcp workingset config my-working-set --get github.timeout

# Get multiple configuration values
docker mcp workingset config my-working-set \
  --get github.timeout \
  --get github.maxRetries

# Get all configuration values
docker mcp workingset config my-working-set --get-all

# Delete configuration values
docker mcp workingset config my-working-set --del github.maxRetries

# Combine operations (set new values and get existing ones)
docker mcp workingset config my-working-set \
  --set github.timeout=60 \
  --get github.maxRetries

# Output in JSON format
docker mcp workingset config my-working-set --get-all --format json

# Output in YAML format
docker mcp workingset config my-working-set --get-all --format yaml
```

**Configuration format:**
- `--set`: Format is `<server-name>.<config-key>=<value>` (can be specified multiple times)
- `--get`: Format is `<server-name>.<config-key>` (can be specified multiple times)
- `--del`: Format is `<server-name>.<config-key>` (can be specified multiple times)
- `--get-all`: Retrieves all configuration values from all servers in the working set
- `--format`: Output format - `human` (default), `json`, or `yaml`

**Important notes:**
- The server name must match the name from the server's snapshot (not the image or source URL)
- Use `docker mcp workingset show <working-set-id> --format yaml` to see available server names
- Configuration changes are persisted immediately to the working set
- You cannot both `--set` and `--del` the same key in a single command
- **Note**: Config is for non-sensitive settings. Use secrets management for API keys, tokens, and passwords.

### Managing Secrets for Working Set Servers

Secrets provide secure storage for sensitive values like API keys, tokens, and passwords. Unlike configuration values, secrets are stored securely and never displayed in plain text.

```bash
# Set a secret for a server in a working set
docker mcp secret set github.pat=ghp_xxxxx
```

**Secret format:**
- Format is `<server-name>.<secret-key>=<value>`
- The server name must match the name from the server's snapshot
- Secrets are stored in Docker Desktop's secure secret store

**Current Limitation**: Secrets are scoped across all servers rather than for each working set. We plan to address this.

### Exporting Working Sets

Export a working set to a file for backup or sharing:

```bash
# Export to YAML
docker mcp workingset export my-working-set ./my-workingset.yaml

# Export to JSON
docker mcp workingset export my-working-set ./my-workingset.json
```

The file format is automatically detected from the extension (`.yaml` or `.json`).

### Importing Working Sets

Import a working set from a file:

```bash
# Import from YAML
docker mcp workingset import ./my-workingset.yaml

# Import from JSON
docker mcp workingset import ./my-workingset.json
```

**Behavior:**
- If a working set with the same ID doesn't exist, it will be created
- If a working set with the same ID exists, it will be updated
- The file format is automatically detected from the extension

### Pushing Working Sets to OCI Registry

Share working sets via OCI registries:

```bash
# Push to a registry
docker mcp workingset push my-working-set docker.io/myorg/my-workingset:latest

# Push to a private registry
docker mcp workingset push my-working-set registry.example.com/team/workingset:v1.0
```

This allows you to:
- Version control your working sets
- Share with team members
- Deploy consistent configurations across environments

### Pulling Working Sets from OCI Registry

Retrieve working sets from OCI registries:

```bash
# Pull from a registry
docker mcp workingset pull docker.io/myorg/my-workingset:latest

# Pull from a private registry
docker mcp workingset pull registry.example.com/team/workingset:v1.0
```

The working set will be imported into your local system and ready to use.

## Using Working Sets with the Gateway

Once you have working sets configured, you can use them to run the MCP gateway:

```bash
# Run gateway with a specific working set
docker mcp gateway run --working-set my-working-set

# The gateway will start with only the servers defined in that working set
```

**Important restrictions:**
- `--working-set` cannot be used with `--servers` flag
- `--working-set` cannot be used with `--enable-all-servers` flag
- These flags are mutually exclusive to ensure clear server selection

**Current limitations:**
- Working sets currently support image-only servers in the gateway
- Configuration and secrets support is being expanded
- Watch mode and dynamic updates are in development

## Using Working Sets with MCP Clients

Connect an MCP client with a specific working set:

```bash
# Connect client with a working set
docker mcp client connect claude -w my-working-set

# Using long form
docker mcp client connect cursor --working-set my-working-set
```

This generates the appropriate client configuration using the servers from your working set.

## Working Set File Format

Working sets are stored in YAML or JSON with the following structure:

### YAML Format

```yaml
version: 1
id: my-working-set
name: My Working Set
servers:
  - type: image
    image: mcp/github:latest
  - type: registry
    source: https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860
    config:
      key: value
    secrets: default
    tools:
      - tool1
      - tool2
secrets:
  default:
    provider: docker-desktop-store
```

### JSON Format

```json
{
  "version": 1,
  "id": "my-working-set",
  "name": "My Working Set",
  "servers": [
    {
      "type": "image",
      "image": "mcp/github:latest"
    },
    {
      "type": "registry",
      "source": "https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860",
      "config": {
        "key": "value"
      },
      "secrets": "default",
      "tools": ["tool1", "tool2"]
    }
  ],
  "secrets": {
    "default": {
      "provider": "docker-desktop-store"
    }
  }
}
```

### Field Descriptions

- **version**: Working set format version (currently `1`)
- **id**: Unique identifier for the working set
- **name**: Human-readable name
- **servers**: Array of server definitions
  - **type**: Either `image` or `registry`
  - **image**: (For type `image`) Docker image reference
  - **source**: (For type `registry`) MCP Registry URL
  - **config**: Optional configuration key-value pairs
  - **secrets**: Optional reference to a secrets configuration
  - **tools**: Optional list of specific tools to enable from this server
- **secrets**: Map of secret configurations
  - **provider**: Currently only `docker-desktop-store` is supported

## Common Workflows

### Development Workflow

```bash
# 1. Create a development working set
docker mcp workingset create --name dev \
  --server docker://mcp/github:latest \
  --server docker://mcp/filesystem:latest

# 2. Test it with the gateway
docker mcp gateway run --working-set dev

# 3. Once satisfied, export for sharing
docker mcp workingset export dev ./dev-workingset.yaml
```

### Team Collaboration

```bash
# Team lead: Create and share
docker mcp workingset create --name team-tools \
  --server docker://mcp/github:latest \
  --server docker://mcp/slack:latest \
  --server docker://mcp/jira:latest

docker mcp workingset push team-tools docker.io/myorg/team-tools:v1.0

# Team members: Pull and use
docker mcp workingset pull docker.io/myorg/team-tools:v1.0
docker mcp gateway run --working-set team-tools
```

### Environment-Specific Configurations

```bash
# Create different working sets for different environments
docker mcp workingset create --name production \
  --server docker://mcp/monitoring:latest \
  --server docker://mcp/logging:latest

docker mcp workingset create --name staging \
  --server docker://mcp/monitoring:latest \
  --server docker://mcp/logging:latest \
  --server docker://mcp/debug:latest

# Run with appropriate environment
docker mcp gateway run --working-set production  # In production
docker mcp gateway run --working-set staging     # In staging
```

### Project-Based Organization

```bash
# Create working sets per project
docker mcp workingset create --name project-a \
  --server docker://mcp/github:latest \
  --server docker://mcp/postgres:latest

docker mcp workingset create --name project-b \
  --server docker://mcp/github:latest \
  --server docker://mcp/mongodb:latest

# Switch between projects easily
docker mcp gateway run --working-set project-a
# ... or ...
docker mcp gateway run --working-set project-b
```

### Versioning Working Sets

```bash
# Create a working set
docker mcp workingset create --name my-tools \
  --server docker://mcp/github:v1.0

# Push version 1.0
docker mcp workingset push my-tools docker.io/myorg/my-tools:1.0

# Update the working set (modify via export/import)
docker mcp workingset export my-tools ./temp.yaml
# ... edit temp.yaml ...
docker mcp workingset import ./temp.yaml

# Push version 1.1
docker mcp workingset push my-tools docker.io/myorg/my-tools:1.1

# Pull specific version when needed
docker mcp workingset pull docker.io/myorg/my-tools:1.0
docker mcp workingset pull docker.io/myorg/my-tools:1.1
```

## Best Practices

### Naming Conventions

- Use descriptive names: `dev-tools`, `production-monitoring`, `team-shared`
- Keep IDs short and memorable: `dev`, `prod`, `team`
- Use versioning in OCI tags: `v1.0`, `v1.1`, `latest`

### Organization Strategies

- **By Environment**: Separate working sets for dev, staging, production
- **By Team**: Different sets for frontend, backend, devops teams
- **By Project**: One working set per major project or application
- **By Use Case**: Sets for debugging, monitoring, development, etc.

### Sharing and Collaboration

- Use OCI registries for team sharing rather than file exports
- Document your working sets in team wikis or READMEs
- Use semantic versioning for working set tags
- Keep a "team-shared" working set for common tools

### Security Considerations

- Always use `docker mcp secret set` for sensitive values (API keys, tokens, passwords)
- Never use `docker mcp workingset config` for secrets - it's for non-sensitive settings only
- Secrets are stored in Docker Desktop's secure secret store
- Use private OCI registries for proprietary server configurations
- Review server references before importing from external sources

## Troubleshooting

### Working Set Not Found

```bash
Error: working set my-set not found
```

**Solution**: Check available working sets with `docker mcp workingset list`

### Feature Not Enabled

```bash
Error: unknown command "workingset" for "docker mcp"
```

**Solution**: Enable the feature with `docker mcp feature enable working-sets`

### Invalid Server Reference

```bash
Error: invalid server value: myserver
```

**Solution**: Ensure server references use either:
- `docker://` prefix for images
- `http://` or `https://` for registry URLs

### Conflicting Flags

```bash
Error: cannot use --working-set with --servers flag
```

**Solution**: Choose either `--working-set` or `--servers`, not both

### ID Already Exists

```bash
Error: working set with id my-set already exists
```

**Solution**: Either:
- Choose a different name/ID
- Remove the existing set first with `docker mcp workingset rm my-set`
- Omit the `--id` flag to auto-generate a unique ID

### Export/Import File Format

```bash
Error: unsupported file extension: .txt, must be .yaml or .json
```

**Solution**: Use `.yaml` or `.json` file extensions

### Invalid Config Format

```bash
Error: invalid config argument: myconfig, expected <serverName>.<configName>=<value>
```

**Solution**: Ensure config arguments follow the correct format:
- For `--set`: `<server-name>.<config-key>=<value>` (e.g., `github.timeout=30`)
- For `--get`: `<server-name>.<config-key>` (e.g., `github.timeout`)
- For `--del`: `<server-name>.<config-key>` (e.g., `github.timeout`)

### Server Not Found in Config Command

```bash
Error: server github not found in working set
```

**Solution**: 
- Use `docker mcp workingset show <working-set-id>` to see available server names
- Ensure you're using the server's name from its snapshot, not the image name or source URL
- Server names are case-sensitive

### Cannot Delete and Set Same Config

```bash
Error: cannot both delete and set the same config value: github.timeout
```

**Solution**: Don't use `--set` and `--del` for the same key in a single command. Run them separately:
```bash
# First delete
docker mcp workingset config my-set --del github.timeout
# Then set (if needed)
docker mcp workingset config my-set --set github.timeout=60
```

## Limitations and Future Enhancements

### Current Limitations

- Gateway support is limited to image-only servers
- No automatic watch/reload when working sets are updated
- Limited to Docker Desktop's secret store for secrets
- No built-in conflict resolution for duplicate server names

### Planned Enhancements

- Full registry support in the gateway
- Integration with catalog management
- Search and discovery features

## Related Documentation

- [MCP Gateway](./mcp-gateway.md) - Running the MCP gateway
- [Catalog Management](./catalog.md) - Managing MCP server catalogs
- [Feature Management](./troubleshooting.md#features) - Enabling/disabling features
- [Security](./security.md) - Security considerations

