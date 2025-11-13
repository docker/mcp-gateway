# Profiles

Profiles allow you to organize and manage collections of MCP servers as a single unit. Unlike catalogs that serve as repositories of available servers, profiles represent specific configurations of servers you want to use together for different purposes or contexts.

## What are Profiles?

A profile is a named collection of MCP servers that can be:
- Created and managed independently of catalogs
- Shared across teams via export/import or OCI registries
- Used to quickly switch between different server configurations

Profiles are decoupled from catalogs, meaning the servers in a profile can come from:
- **MCP Registry references**: HTTP(S) URLs pointing to servers in the Model Context Protocol registry
- **OCI image references**: Docker images with the `docker://` prefix

## Enabling Profiles

Profiles are a feature that must be enabled first:

```bash
# Enable the profiles feature
docker mcp feature enable profiles

# Verify it's enabled
docker mcp feature list
```

Once enabled, you'll have access to:
- `docker mcp profile` commands for managing profiles
- `--profile` flag for `docker mcp gateway run`
- `-p` or `--profile` flag for `docker mcp client connect`

## Profile Commands

### Creating Profiles

Create a new profile with a name and list of servers:

```bash
# Create a profile with OCI image references
docker mcp profile create --name dev-tools \
  --server docker://mcp/github:latest \
  --server docker://mcp/slack:latest

# Create with MCP Registry references
docker mcp profile create --name registry-servers \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

# Mix MCP Registry and OCI references
docker mcp profile create --name mixed \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 \
  --server docker://mcp/github:latest

# Specify a custom ID (otherwise derived from name)
docker mcp profile create --name "My Servers" --id my-servers \
  --server docker://mcp/github:latest
```

**Notes:**
- `--name` is required and serves as the human-readable name
- `--id` is optional; if not provided, it's generated from the name (lowercase, alphanumeric with hyphens)
- `--server` can be specified multiple times to add multiple servers
- Server references must be either:
  - `docker://` prefix for OCI images
  - `http://` or `https://` URLs for MCP Registry references

### Listing Profiles

View all profiles in your system:

```bash
# List all profiles (human-readable format)
docker mcp profile list

# List with aliases
docker mcp profile ls

# List in JSON format
docker mcp profile list --format json

# List in YAML format
docker mcp profile list --format yaml
```

**Output formats:**
- `human` (default): Tabular format with ID and Name columns
- `json`: Complete JSON representation
- `yaml`: Complete YAML representation

### Showing Profile Details

Display detailed information about a specific profile:

```bash
# Show a profile (human-readable)
docker mcp profile show my-profile

# Show in JSON format
docker mcp profile show my-profile --format json

# Show in YAML format
docker mcp profile show my-profile --format yaml
```

The output includes:
- Profile ID and name
- List of servers with their types and references
- Configuration for each server
- Secrets configuration
- Tools associated with each server

### Removing Profiles

Delete a profile from your system:

```bash
# Remove a profile
docker mcp profile remove my-profile

# Using alias
docker mcp profile rm my-profile
```

**Note:** This only removes the profile definition, not the actual server images or registry entries.

### Configuring Profile Servers

Manage configuration values for servers within a profile:

```bash
# Set a single configuration value
docker mcp profile config my-profile --set github.timeout=30

# Set multiple configuration values
docker mcp profile config my-profile \
  --set github.timeout=30 \
  --set github.maxRetries=3 \
  --set slack.channel=general

# Get a specific configuration value
docker mcp profile config my-profile --get github.timeout

# Get multiple configuration values
docker mcp profile config my-profile \
  --get github.timeout \
  --get github.maxRetries

# Get all configuration values
docker mcp profile config my-profile --get-all

# Delete configuration values
docker mcp profile config my-profile --del github.maxRetries

# Combine operations (set new values and get existing ones)
docker mcp profile config my-profile \
  --set github.timeout=60 \
  --get github.maxRetries

# Output in JSON format
docker mcp profile config my-profile --get-all --format json

# Output in YAML format
docker mcp profile config my-profile --get-all --format yaml
```

**Configuration format:**
- `--set`: Format is `<server-name>.<config-key>=<value>` (can be specified multiple times)
- `--get`: Format is `<server-name>.<config-key>` (can be specified multiple times)
- `--del`: Format is `<server-name>.<config-key>` (can be specified multiple times)
- `--get-all`: Retrieves all configuration values from all servers in the profile
- `--format`: Output format - `human` (default), `json`, or `yaml`

**Important notes:**
- The server name must match the name from the server's snapshot (not the image or source URL)
- Use `docker mcp profile show <profile-id> --format yaml` to see available server names
- Configuration changes are persisted immediately to the profile
- You cannot both `--set` and `--del` the same key in a single command
- **Note**: Config is for non-sensitive settings. Use secrets management for API keys, tokens, and passwords.

### Managing Secrets for Profile Servers

Secrets provide secure storage for sensitive values like API keys, tokens, and passwords. Unlike configuration values, secrets are stored securely and never displayed in plain text.

```bash
# Set a secret for a server in a profile
docker mcp secret set github.pat=ghp_xxxxx
```

**Secret format:**
- Format is `<server-name>.<secret-key>=<value>`
- The server name must match the name from the server's snapshot
- Secrets are stored in Docker Desktop's secure secret store

**Current Limitation**: Secrets are scoped across all servers rather than for each profile. We plan to address this.

### Exporting Profiles

Export a profile to a file for backup or sharing:

```bash
# Export to YAML
docker mcp profile export my-profile ./my-profile.yaml

# Export to JSON
docker mcp profile export my-profile ./my-profile.json
```

The file format is automatically detected from the extension (`.yaml` or `.json`).

### Importing Profiles

Import a profile from a file:

```bash
# Import from YAML
docker mcp profile import ./my-profile.yaml

# Import from JSON
docker mcp profile import ./my-profile.json
```

**Behavior:**
- If a profile with the same ID doesn't exist, it will be created
- If a profile with the same ID exists, it will be updated
- The file format is automatically detected from the extension

### Pushing Profiles to OCI Registry

Share profiles via OCI registries:

```bash
# Push to a registry
docker mcp profile push my-profile docker.io/myorg/my-profile:latest

# Push to a private registry
docker mcp profile push my-profile registry.example.com/team/profile:v1.0
```

This allows you to:
- Version control your profiles
- Share with team members
- Deploy consistent configurations across environments

### Pulling Profiles from OCI Registry

Retrieve profiles from OCI registries:

```bash
# Pull from a registry
docker mcp profile pull docker.io/myorg/my-profile:latest

# Pull from a private registry
docker mcp profile pull registry.example.com/team/profile:v1.0
```

The profile will be imported into your local system and ready to use.

## Using Profiles with the Gateway

Once you have profiles configured, you can use them to run the MCP gateway:

```bash
# Run gateway with a specific profile
docker mcp gateway run --profile my-profile

# The gateway will start with only the servers defined in that profile
```

**Important restrictions:**
- `--profile` cannot be used with `--servers` flag
- `--profile` cannot be used with `--enable-all-servers` flag
- These flags are mutually exclusive to ensure clear server selection

**Current limitations:**
- Profiles currently support image-only servers in the gateway
- Configuration and secrets support is being expanded
- Watch mode and dynamic updates are in development

## Using Profiles with MCP Clients

Connect an MCP client with a specific profile:

```bash
# Connect client with a profile
docker mcp client connect claude -p my-profile

# Using long form
docker mcp client connect cursor --profile my-profile
```

This generates the appropriate client configuration using the servers from your profile.

## Profile File Format

Profiles are stored in YAML or JSON with the following structure:

### YAML Format

```yaml
version: 1
id: my-profile
name: My Profile
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
  "id": "my-profile",
  "name": "My Profile",
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

- **version**: Profile format version (currently `1`)
- **id**: Unique identifier for the profile
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
# 1. Create a development profile
docker mcp profile create --name dev \
  --server docker://mcp/github:latest \
  --server docker://mcp/filesystem:latest

# 2. Test it with the gateway
docker mcp gateway run --profile dev

# 3. Once satisfied, export for sharing
docker mcp profile export dev ./dev-profile.yaml
```

### Team Collaboration

```bash
# Team lead: Create and share
docker mcp profile create --name team-tools \
  --server docker://mcp/github:latest \
  --server docker://mcp/slack:latest \
  --server docker://mcp/jira:latest

docker mcp profile push team-tools docker.io/myorg/team-tools:v1.0

# Team members: Pull and use
docker mcp profile pull docker.io/myorg/team-tools:v1.0
docker mcp gateway run --profile team-tools
```

### Environment-Specific Configurations

```bash
# Create different profiles for different environments
docker mcp profile create --name production \
  --server docker://mcp/monitoring:latest \
  --server docker://mcp/logging:latest

docker mcp profile create --name staging \
  --server docker://mcp/monitoring:latest \
  --server docker://mcp/logging:latest \
  --server docker://mcp/debug:latest

# Run with appropriate environment
docker mcp gateway run --profile production  # In production
docker mcp gateway run --profile staging     # In staging
```

### Project-Based Organization

```bash
# Create profiles per project
docker mcp profile create --name project-a \
  --server docker://mcp/github:latest \
  --server docker://mcp/postgres:latest

docker mcp profile create --name project-b \
  --server docker://mcp/github:latest \
  --server docker://mcp/mongodb:latest

# Switch between projects easily
docker mcp gateway run --profile project-a
# ... or ...
docker mcp gateway run --profile project-b
```

### Versioning Profiles

```bash
# Create a profile
docker mcp profile create --name my-tools \
  --server docker://mcp/github:v1.0

# Push version 1.0
docker mcp profile push my-tools docker.io/myorg/my-tools:1.0

# Update the profile (modify via export/import)
docker mcp profile export my-tools ./temp.yaml
# ... edit temp.yaml ...
docker mcp profile import ./temp.yaml

# Push version 1.1
docker mcp profile push my-tools docker.io/myorg/my-tools:1.1

# Pull specific version when needed
docker mcp profile pull docker.io/myorg/my-tools:1.0
docker mcp profile pull docker.io/myorg/my-tools:1.1
```

## Best Practices

### Naming Conventions

- Use descriptive names: `dev-tools`, `production-monitoring`, `team-shared`
- Keep IDs short and memorable: `dev`, `prod`, `team`
- Use versioning in OCI tags: `v1.0`, `v1.1`, `latest`

### Organization Strategies

- **By Environment**: Separate profiles for dev, staging, production
- **By Team**: Different sets for frontend, backend, devops teams
- **By Project**: One profile per major project or application
- **By Use Case**: Sets for debugging, monitoring, development, etc.

### Sharing and Collaboration

- Use OCI registries for team sharing rather than file exports
- Document your profiles in team wikis or READMEs
- Use semantic versioning for profile tags
- Keep a "team-shared" profile for common tools

### Security Considerations

- Always use `docker mcp secret set` for sensitive values (API keys, tokens, passwords)
- Never use `docker mcp profile config` for secrets - it's for non-sensitive settings only
- Secrets are stored in Docker Desktop's secure secret store
- Use private OCI registries for proprietary server configurations
- Review server references before importing from external sources

## Troubleshooting

### Profile Not Found

```bash
Error: profile my-set not found
```

**Solution**: Check available profiles with `docker mcp profile list`

### Feature Not Enabled

```bash
Error: unknown command "profile" for "docker mcp"
```

**Solution**: Enable the feature with `docker mcp feature enable profiles`

### Invalid Server Reference

```bash
Error: invalid server value: myserver
```

**Solution**: Ensure server references use either:
- `docker://` prefix for images
- `http://` or `https://` for registry URLs

### Conflicting Flags

```bash
Error: cannot use --profile with --servers flag
```

**Solution**: Choose either `--profile` or `--servers`, not both

### ID Already Exists

```bash
Error: profile with id my-set already exists
```

**Solution**: Either:
- Choose a different name/ID
- Remove the existing set first with `docker mcp profile rm my-set`
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
Error: server github not found in profile
```

**Solution**: 
- Use `docker mcp profile show <profile-id>` to see available server names
- Ensure you're using the server's name from its snapshot, not the image name or source URL
- Server names are case-sensitive

### Cannot Delete and Set Same Config

```bash
Error: cannot both delete and set the same config value: github.timeout
```

**Solution**: Don't use `--set` and `--del` for the same key in a single command. Run them separately:
```bash
# First delete
docker mcp profile config my-set --del github.timeout
# Then set (if needed)
docker mcp profile config my-set --set github.timeout=60
```

## Limitations and Future Enhancements

### Current Limitations

- Gateway support is limited to image-only servers
- No automatic watch/reload when profiles are updated
- Limited to Docker Desktop's secret store for secrets
- No built-in conflict resolution for duplicate server names

### Planned Enhancements

- Full registry support in the gateway
- Integration with catalog management
- Search and discovery features

## Creating Catalogs from Profiles

The `catalog-next` command allows you to create and share catalogs:

```bash
# Create a catalog from a profile
docker mcp catalog-next create --from-profile my-profile

# Create with a custom name
docker mcp catalog-next create --from-profile my-profile --name "My Catalog"

# List all catalogs
docker mcp catalog-next list

# Show catalog details
docker mcp catalog-next show <catalog-digest>

# Remove a catalog
docker mcp catalog-next remove <catalog-digest>

# Push catalog to OCI registry
docker mcp catalog-next push <catalog-digest> myorg/my-catalog:latest

# Pull catalog from OCI registry
docker mcp catalog-next pull myorg/my-catalog:latest
```

**Key points:**
- Catalogs are an immutable collection of MCP Servers
- When creating a catalog from a profile, only the servers are included in the catalog.
- Use catalogs to share stable server configurations across teams
- Catalogs can be pushed to/pulled from OCI registries like Docker images
- Output supports `--format` flag: `human` (default), `json`, or `yaml`

## Related Documentation

- [MCP Gateway](./mcp-gateway.md) - Running the MCP gateway
- [Catalog Management](./catalog.md) - Managing MCP server catalogs
- [Feature Management](./troubleshooting.md#features) - Enabling/disabling features
- [Security](./security.md) - Security considerations

