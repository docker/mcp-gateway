# MCP Catalog Management

Docker MCP Gateway provides comprehensive catalog management capabilities, allowing you to create, manage, and use custom MCP server catalogs alongside Docker's official catalog.

## Quick Start with Bootstrap

The easiest way to get started with custom catalogs is to use the `bootstrap` command, which creates a starter catalog file with Docker's official server entries as examples:

```bash
# Create a starter catalog with Docker Hub and Docker CLI server examples
docker mcp catalog bootstrap ./my-starter-catalog.yaml

# The file now contains properly formatted server definitions you can modify
```

This creates a YAML file with real server definitions that you can:
- Modify to understand the catalog format
- Use as a foundation for your custom servers
- Import directly into your catalog collection
- Use as a source for copying individual servers to other catalogs

## Feature Activation

Custom catalogs can be managed by enabling the `configured-catalogs` feature:

```bash
# Enable the feature
docker mcp feature enable configured-catalogs

# Verify it's enabled
docker mcp feature list

# Use configured catalogs with the gateway
docker mcp gateway run --use-configured-catalogs
```

## Catalog Management Commands

### Listing Catalogs

```bash
# List all configured catalogs
docker mcp catalog ls

# List in JSON format
docker mcp catalog ls --json
```

### Creating Catalogs

```bash
# Create a new empty catalog
docker mcp catalog create my-custom-catalog

# The catalog is now ready for adding servers
```

### Viewing Catalog Contents

```bash
# Show servers in the default Docker catalog
docker mcp catalog show

# Show servers in a specific catalog
docker mcp catalog show my-custom-catalog

# Show in different formats
docker mcp catalog show docker-mcp --format json
docker mcp catalog show docker-mcp --format yaml
```

### Adding Servers to Catalogs

```bash
# Add a server from another catalog file
docker mcp catalog add my-custom-catalog server-name ./source-catalog.yaml

# Force overwrite if server already exists
docker mcp catalog add my-custom-catalog server-name ./source-catalog.yaml --force
```

### Importing Catalogs

```bash
# Import a catalog from a local file
docker mcp catalog import ./my-catalog.yaml

# Import from a URL
docker mcp catalog import https://example.com/catalog.yaml

# Import with an alias
docker mcp catalog import team-servers
```

### Exporting Catalogs

```bash
# Export a custom catalog to a file
docker mcp catalog export my-custom-catalog ./backup.yaml

# Note: You cannot export Docker's official catalog
docker mcp catalog export docker-mcp ./docker-backup.yaml
# Error: Cannot export the Docker MCP catalog as it is managed by Docker
```

### Forking Catalogs

```bash
# Create a copy of an existing catalog
docker mcp catalog fork docker-mcp my-custom-version

# Now you can modify my-custom-version independently
```

### Removing Catalogs

```bash
# Remove a custom catalog
docker mcp catalog rm my-custom-catalog

# Note: You cannot remove Docker's official catalog
```

### Updating Catalogs

```bash
# Update all catalogs
docker mcp catalog update

# Update a specific catalog
docker mcp catalog update my-custom-catalog
```

### Resetting Catalogs

```bash
# Remove all custom catalogs and reset to Docker defaults
docker mcp catalog reset
```

## Catalog YAML Format

Catalogs use a specific YAML format with server definitions. Here's the structure:

### Basic Structure

```yaml
name: my-catalog
displayName: My Custom Catalog
registry:
  server-name:
    description: "Description of what this server does"
    title: "Server Display Name"
    type: "server"  # or "poci" for container tools
    image: "namespace/image:tag"
    tools:
      - name: "tool1"
      - name: "tool2"
    # Additional server configuration...
```

### Complete Server Example

```yaml
registry:
  my-custom-server:
    description: "My custom MCP server for database operations"
    title: "Database Helper"
    type: "server"
    dateAdded: "2025-08-01T00:00:00Z"
    image: "myorg/db-server:latest"
    
    # Tools provided by this server
    tools:
      - name: "query_database"
      - name: "create_table"
      - name: "backup_data"
    
    # Required secrets
    secrets:
      - name: "my-custom-server.api_key"
        env: "DB_API_KEY"
        example: "your-api-key-here"
    
    # Environment variables
    env:
      - name: "DB_HOST"
        value: "{{my-custom-server.host}}"
      - name: "DB_PORT"
        value: "{{my-custom-server.port}}"
    
    # Command line arguments
    command:
      - "--transport=stdio"
      - "--config={{my-custom-server.config_path}}"
    
    # Volume mounts
    volumes:
      - "{{my-custom-server.data_path}}:/data"
    
    # Configuration schema
    config:
      - name: "my-custom-server"
        description: "Database server configuration"
        type: "object"
        properties:
          host:
            type: "string"
          port:
            type: "string"
          config_path:
            type: "string"
          data_path:
            type: "string"
        required: ["host", "port"]
    
    # Metadata
    metadata:
      category: "database"
      tags: ["database", "sql", "backup"]
      license: "MIT License"
      owner: "myorg"
    
    # Documentation links
    readme: "https://github.com/myorg/db-server/README.md"
    source: "https://github.com/myorg/db-server"
    upstream: "https://github.com/myorg/db-server"
    icon: "https://avatars.githubusercontent.com/u/myorg"
```

### POCI (Container) Tool Example

```yaml
registry:
  my-container-tool:
    description: "A containerized tool for file processing"
    title: "File Processor"
    type: "poci"
    tools:
      - name: "process_files"
        description: "Process files in a directory"
        parameters:
          type: "object"
          properties:
            input_path:
              type: "string"
              description: "Path to input files"
            output_path:
              type: "string"
              description: "Path for output files"
          required: ["input_path", "output_path"]
        container:
          image: "myorg/file-processor:latest"
          command:
            - "process"
            - "{{input_path}}"
            - "{{output_path}}"
          volumes:
            - "{{input_path}}:{{input_path}}"
            - "{{output_path}}:{{output_path}}"
```

## Common Workflows

### Development Workflow

```bash
# 1. Create a starter catalog for reference
docker mcp catalog bootstrap ./starter-catalog.yaml

# 2. Create your own catalog
docker mcp catalog create dev-servers

# 3. Add useful servers from the starter (optional)
docker mcp catalog add dev-servers dockerhub ./starter-catalog.yaml

# 4. Create a custom server definition file
cat > my-server.yaml << EOF
registry:
  my-dev-server:
    description: "My development server"
    title: "Dev Server"
    type: "server"
    image: "myorg/dev-server:latest"
    tools:
      - name: "dev_tool"
EOF

# 5. Add your custom server
docker mcp catalog add dev-servers my-dev-server ./my-server.yaml

# 6. Enable the feature and run with your catalog
docker mcp feature enable configured-catalogs
docker mcp gateway run --use-configured-catalogs
```

### Team Sharing Workflow

```bash
# Team lead: Create and export a team catalog
docker mcp catalog create team-servers
docker mcp catalog add team-servers shared-db ./shared-db-server.yaml
docker mcp catalog add team-servers api-helper ./api-helper-server.yaml
docker mcp catalog export team-servers ./team-catalog.yaml

# Share team-catalog.yaml with team members

# Team members: Import the shared catalog
docker mcp catalog import ./team-catalog.yaml
docker mcp feature enable configured-catalogs
docker mcp gateway run --use-configured-catalogs
```

### Testing New Servers

```bash
# 1. Create a test catalog
docker mcp catalog create test-servers

# 2. Add your server for testing
docker mcp catalog add test-servers test-server ./test-server.yaml

# 3. Run gateway with test catalog
docker mcp gateway run --use-configured-catalogs

# 4. When done testing, clean up
docker mcp catalog rm test-servers
```

### Production Deployment

```bash
# 1. Create production catalog
docker mcp catalog create prod-servers

# 2. Add only production-ready servers
docker mcp catalog add prod-servers monitoring ./monitoring-server.yaml
docker mcp catalog add prod-servers logging ./logging-server.yaml

# 3. Export for backup and deployment
docker mcp catalog export prod-servers ./prod-catalog-backup.yaml

# 4. Deploy with production catalog
docker mcp feature enable configured-catalogs
docker mcp gateway run --use-configured-catalogs
```

## Catalog Precedence

When using `--use-configured-catalogs`, catalogs are loaded in this order:

1. **Docker Official Catalog** (always loaded first)
2. **Configured Catalogs** (user-imported catalogs)
3. **CLI-specified Catalogs** (via `--catalog` flag)

If multiple catalogs define the same server name, the **last-loaded catalog wins**. This means CLI-specified catalogs take highest precedence, followed by configured catalogs, with Docker's catalog as the base.

## Registry Submission

For servers you want to share with the broader community, consider submitting them to Docker's official registry:

**Official Docker MCP Registry**: https://github.com/docker/mcp-registry

This process makes your servers available to all Docker MCP users through the official catalog.

## Troubleshooting

### Feature Not Enabled
```bash
Error: configured catalogs feature is not enabled
```
**Solution**: Run `docker mcp feature enable configured-catalogs`

### File Already Exists
```bash
Error: file "catalog.yaml" already exists - will not overwrite
```
**Solution**: Choose a different filename or move/remove the existing file

### Server Not Found
```bash
Error: server "server-name" not found in catalog "source.yaml"
```
**Solution**: Check the server name spelling and verify it exists in the source catalog

### Cannot Export Docker Catalog
```bash
Error: cannot export the Docker MCP catalog as it is managed by Docker
```
**Solution**: This is intentional - Docker's catalog cannot be exported. Use `bootstrap` to get Docker server examples instead.

### Container Mode Issues
If running in container mode, ensure proper volume mounts for accessing catalog files and Docker configuration.