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

⚠️ **Important Caveat:** MCP Registry references are not fully implemented and are not expected to work yet.

## Enabling Profiles

Profiles are a feature that must be enabled first.

In Docker CE or container mode:

```bash
# Enable the profiles feature
docker mcp feature enable profiles

# Verify it's enabled
docker mcp feature list
```

If you're using Docker Desktop, you have to enable the `MCPWorkingSets` feature flag within Docker Desktop instead.

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
  --server docker://my-server:latest \
  --server docker://my-other-server:latest

# Create with MCP Registry references
docker mcp profile create --name registry-servers \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

# Mix MCP Registry and OCI references
docker mcp profile create --name mixed \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 \
  --server docker://my-server:latest

# Specify a custom ID (otherwise derived from name)
docker mcp profile create --name "My Servers" --id my-servers \
  --server docker://my-server:latest
```

**Notes:**
- `--name` is required and serves as the human-readable name
- `--id` is optional; if not provided, it's generated from the name (lowercase, alphanumeric with hyphens)
- `--server` can be specified multiple times to add multiple servers
- Server references must be either:
  - `docker://` prefix for OCI images
  - `http://` or `https://` URLs for MCP Registry references

### Adding Servers to a Profile

After creating a profile, you can add more servers to it:

```bash
# Add servers with OCI references
docker mcp profile server add dev-tools \
  --server docker://my-server:latest \
  --server docker://my-other-server:latest

# Add servers with MCP Registry references
docker mcp profile server add dev-tools \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860

# Mix MCP Registry references and OCI references
docker mcp profile server add dev-tools \
  --server https://registry.modelcontextprotocol.io/v0/servers/71de5a2a-6cfb-4250-a196-f93080ecc860 \
  --server docker://my-server:latest

# Add servers from a catalog
docker mcp profile server add dev-tools \
  --server catalog://mcp/docker-mcp-catalog/github+slack

# Mix catalog servers with direct server references
docker mcp profile server add dev-tools \
  --server catalog://mcp/docker-mcp-catalog/github
  --server docker://my-server:latest
```

**Server References:**
- Use `--server` flag for all server references (can be specified multiple times)
- Server references must start with:
  - `catalog://` for catalog references. This takes the form of `catalog://<catalog-oci-refence>/<server-1>+<server-2>
  - `docker://` for OCI images
  - `http://` or `https://` for MCP Registry URLs
- Catalog servers are referenced by their name within the catalog

**Notes:**
- You can add multiple servers in a single command
- You can mix direct server references with catalog-based references
- If a server already exists in the profile, the operation will skip it or update it

### Removing Servers from a Profile

Remove servers from a profile by their server name:

```bash
# Remove servers by name
docker mcp profile server remove dev-tools \
  --name github \
  --name slack

# Remove a single server
docker mcp profile server remove dev-tools --name github

# Using alias
docker mcp profile server rm dev-tools --name github
```

**Server Names:**
- Use `--name` flag to specify server names to remove (can be specified multiple times)
- Server names are determined by the server's snapshot (not the image name or source URL)
- Use `docker mcp profile show <profile-id>` to see available server names in a profile

### Listing Servers Across Profiles

View all servers grouped by profile, with filtering capabilities:

```bash
# List all servers across all profiles
docker mcp profile servers

# Filter servers by name (case-insensitive substring matching)
docker mcp profile servers --filter github

# Show servers from a specific profile only
docker mcp profile servers --profile dev-tools

# Combine filter and profile
docker mcp profile servers --profile dev-tools --filter slack

# Output in JSON format
docker mcp profile servers --format json

# Output in YAML format
docker mcp profile servers --format yaml
```

**Output options:**
- `--filter`: Search for servers matching a query (case-insensitive substring matching on image names or source URLs)
- `--profile` or `-p`: Show servers only from a specific profile
- `--format`: Output format - `human` (default), `json`, or `yaml`

**Notes:**
- This command provides a global view of all servers across your profiles
- Useful for finding which profiles contain specific servers
- The filter applies to both image names and source URLs

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

**Typed values via JSON:**

- Values passed to `--set` are parsed as JSON when possible. This allows arrays, numbers, booleans and objects.
- If the value is not valid JSON, it is stored as a plain string.
- Examples:

```bash
# Numbers and booleans
docker mcp profile config my-profile --set github.timeout=60 --set github.debug=true

# Strings (either raw or JSON-quoted both work)
docker mcp profile config my-profile --set slack.channel=general
docker mcp profile config my-profile --set slack.channel='"general"'

# Arrays
docker mcp profile config my-profile --set filesystem.paths='["/Users/dk/dev","/Users/dk/projects"]'

# Objects
docker mcp profile config my-profile --set build.options='{"retries":3,"cache":false}'
```

**Important notes:**
- The server name must match the name from the server's snapshot (not the image or source URL)
- Use `docker mcp profile show <profile-id> --format yaml` to see available server names
- Configuration changes are persisted immediately to the profile
- You cannot both `--set` and `--del` the same key in a single command
- **Note**: Config is for non-sensitive settings. Use secrets management for API keys, tokens, and passwords.

### Managing Tools for Profile Servers

Control which tools are enabled or disabled for servers in a profile:

```bash
# Enable specific tools for a server
docker mcp profile tools my-profile \
  --enable github.create_issue \
  --enable github.list_repos

# Disable specific tools for a server
docker mcp profile tools my-profile \
  --disable github.create_issue \
  --disable github.search_code

# Enable and disable in one command
docker mcp profile tools my-profile \
  --enable github.create_issue \
  --disable github.search_code

# Enable all tools for a server
docker mcp profile tools my-profile --enable-all github

# Disable all tools for a server
docker mcp profile tools my-profile --disable-all github

# View all enabled tools in the profile
docker mcp profile show my-profile
```

**Tool management format:**
- `--enable`: Format is `<server-name>.<tool-name>` (can be specified multiple times)
- `--disable`: Format is `<server-name>.<tool-name>` (can be specified multiple times)
- `--enable-all`: Format is `<server-name>` to enable all tools for a server (can be specified multiple times)
- `--disable-all`: Format is `<server-name>` to disable all tools for a server (can be specified multiple times)

**Important notes:**
- Tool names use dot notation: `<serverName>.<toolName>`
- The server name must match the name from the server's snapshot
- Use `docker mcp profile show <profile-id>` to see which tools are currently enabled
- By default, all tools are enabled unless explicitly disabled
- Changes take effect immediately and persist in the profile

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

# Connect upon creating the profile
docker mcp profile create --name my-profile --connect cursor
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
  --server catalog://mcp/docker-mcp-catalog:latest/filesystem

# 2. Test it with the gateway
docker mcp gateway run --profile dev

# 3. Add more servers as needed
docker mcp profile server add dev \
  --server catalog://mcp/docker-mcp-catalog:latest/slack

# 4. Remove servers you don't need
docker mcp profile server remove dev --name filesystem

# 5. Disable dangerous tools in filesystem server for safety
docker mcp profile tools dev --disable filesystem.delete_file

# 6. View all servers across profiles to check your setup
docker mcp profile servers

# 7. Once satisfied, export for sharing
docker mcp profile export dev ./dev-profile.yaml
```

### Team Collaboration

```bash
# Team lead: Create and share
docker mcp profile create --name team-tools \
  --server docker://my-server:latest \
  --server docker://my-other-server:latest \
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
  --server docker://monitoring:latest \
  --server docker://logging:latest

docker mcp profile create --name staging \
  --server docker://monitoring:latest \
  --server docker://logging:latest \
  --server docker://debug:latest

# Run with appropriate environment
docker mcp gateway run --profile production  # In production
docker mcp gateway run --profile staging     # In staging
```

### Project-Based Organization

```bash
# Create profiles per project
docker mcp profile create --name project-a \
  --server docker://my-server:latest \
  --server docker://postgres:latest

docker mcp profile create --name project-b \
  --server docker://my-server:latest \
  --server docker://mongodb:latest

# Switch between projects easily
docker mcp gateway run --profile project-a
# ... or ...
docker mcp gateway run --profile project-b
```

### Versioning Profiles

```bash
# Create a profile
docker mcp profile create --name my-tools \
  --server docker://my-server:v1.0

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

### Building Profiles from Catalogs

```bash
# 1. Import Docker's official catalog (or pull from OCI registry)
docker mcp catalog create docker-mcp-catalog \
  --from-legacy-catalog https://desktop.docker.com/mcp/catalog/v3/catalog.json

# Or pull a team catalog from OCI registry
docker mcp catalog pull myorg/team-catalog:latest

# 2. Create an initial profile
docker mcp profile create --name my-workflow

# 3. Add specific servers from Docker's official catalog
docker mcp profile server add my-workflow \
  --server catalog://docker-mcp-catalog/github+slack

# 4. Optionally add servers from another catalog or direct references
docker mcp profile server add my-workflow \
  --server catalog://my-org/team-catalog/custom-tool

# Or add a direct OCI reference
docker mcp profile server add my-workflow \
  --server docker://mcp/custom-tool:latest

# 5. Configure and use
docker mcp profile config my-workflow --set github.timeout=30
docker mcp gateway run --profile my-workflow
```

### Fine-Tuning Tool Access

```bash
# Create a production profile with restricted tool access
docker mcp profile create --name production \
  --server docker://my-server:latest \
  --server docker://my-other-server:latest

# Disable all tools first, then enable only what's needed
docker mcp profile tools production --disable-all github --disable-all slack

# Enable only safe, read-only tools for GitHub
docker mcp profile tools production \
  --enable github.list_repos \
  --enable github.get_file \
  --enable github.search_code

# Enable only message sending for Slack (no channel management)
docker mcp profile tools production \
  --enable slack.send_message \
  --enable slack.list_channels

# Verify the tool configuration
docker mcp profile show production --format yaml

# Use the restricted profile
docker mcp gateway run --profile production
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
- Use `docker mcp profile tools` to disable dangerous or unnecessary tools in production
- Apply the principle of least privilege: enable only the tools actually needed
- Create separate profiles for different security contexts (dev vs. production)

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

**Solution**: In Docker CE or container mode, enable the feature with `docker mcp feature enable profiles`. In Docker Desktop, enable the `MCPWorkingSets` feature flag.

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

### Server Not Found in Catalog

```bash
Error: server 'nonexistent' not found in catalog
```

**Solution**: 
- Use `docker mcp catalog show <catalog-name>` to see available servers in the catalog
- Check that the server name is spelled correctly (names are case-sensitive)

### Cannot Remove Server

```bash
Error: server 'github' not found in profile
```

**Solution**:
- Use `docker mcp profile show <profile-id> --format yaml` to see current servers in the profile
- Ensure you're using the correct server name in the snapshot (not the image name or source URL)
- Server names are case-sensitive

### Invalid Tool Name Format

```bash
Error: invalid tool specification: github
```

**Solution**: Tool names must use dot notation with both server and tool name:
```bash
# Wrong
docker mcp profile tools my-profile --enable github

# Correct
docker mcp profile tools my-profile --enable github.create_issue
```

### Server Not Found When Managing Tools

```bash
Error: server 'github' not found in profile
```

**Solution**:
- Ensure the server with that snapshot name exists in the profile: `docker mcp profile show <profile-id> --format yaml`
- Server names are case-sensitive and must match the snapshot name
- Add the server first if it doesn't exist: `docker mcp profile server add <profile-id> --server docker://my-org/my-server`

### Tool Not Found in Server

```bash
Error: tool 'invalid_tool' not found in server 'github'
```

**Solution**:
- Use `docker mcp profile show <profile-id> --format yaml` to see available tools for each server
- Tool names are case-sensitive and must match exactly
- Check the server's documentation for available tool names

## Limitations and Future Enhancements

### Current Limitations

- Gateway support is limited to image-only servers
- No automatic watch/reload when profiles are updated
- Limited to Docker Desktop's secret store for secrets
- No built-in conflict resolution for duplicate server names

### Planned Enhancements

- Full registry support in the gateway
- Search and discovery features

## Creating Catalogs

The `catalog` command allows you to create and share catalogs:

```bash
# Create a catalog from a working set
docker mcp catalog create my-catalog --from-profile my-profile

# Create with a custom name
docker mcp catalog create my-catalog --from-profile my-profile --name "My Catalog"

# Create a catalog from a legacy catalog
docker mcp catalog create docker-mcp-catalog --from-legacy-catalog https://desktop.docker.com/mcp/catalog/v3/catalog.json

# Create a catalog with servers from other catalogs
docker mcp catalog create dev-catalog --title dev-tools --server catalog://mcp/docker-mcp-catalog/github+notion+obsidian

# Create a catalog with your own MCP server
docker mcp catalog create my-catalog --title my-catalog --server docker://my-server:latest

# List all catalogs
docker mcp catalog list

# Show catalog details
docker mcp catalog show my-catalog

# Show a catalog, pulling it if missing (other options include 'never' and 'always')
docker mcp catalog show mcp/docker-mcp-catalog --pull missing

# Remove a catalog
docker mcp catalog remove my-catalog

# Push catalog to OCI registry
docker mcp catalog tag my-catalog my-org/my-catalog:latest
docker mcp catalog push myorg/my-catalog:latest

# Pull catalog from OCI registry
docker mcp catalog pull myorg/my-catalog:latest

# List servers in a catalog
docker mcp catalog server ls mcp/docker-mcp-catalog:latest

# Filter servers by name
docker mcp catalog server ls mcp/docker-mcp-catalog:latest --filter name=github

# Inspect a specific server in a catalog
docker mcp catalog server inspect mcp/docker-mcp-catalog:latest github

# Inspect in JSON format
docker mcp catalog server inspect mcp/docker-mcp-catalog:latest github --format json
```

**Key points:**
- Catalogs are an immutable collection of MCP Servers
- When creating a catalog from a profile, only the servers are included in the catalog.
- Use catalogs to share a stable server collection across teams
- Catalogs can be pushed to/pulled from OCI registries like Docker images
- Output supports `--format` flag: `human` (default), `json`, or `yaml`

**💡 Tip:** You can import Docker's official MCP catalog as a starting point:
```bash
docker mcp catalog create docker-mcp-catalog \
  --from-legacy-catalog https://desktop.docker.com/mcp/catalog/v3/catalog.json
```
This gives you access to Docker's curated collection of MCP servers, which you can then use to build your profiles with the `--server catalog://docker-mcp-catalog/<server>` flag

## Related Documentation

- [MCP Gateway](./mcp-gateway.md) - Running the MCP gateway
- [Catalog Management](./catalog.md) - Managing MCP server catalogs
- [Feature Management](./troubleshooting.md#features) - Enabling/disabling features
- [Security](./security.md) - Security considerations

