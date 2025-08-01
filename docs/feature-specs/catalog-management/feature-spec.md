# User-Managed Catalogs Feature Specification

## Overview

Enable Docker MCP Gateway users to create, manage, and automatically use custom MCP server catalogs alongside the default Docker catalog, while maintaining backward compatibility with Docker Desktop.

## Background

### Current State
- **Docker Official Catalog**: Hardcoded catalog at `https://desktop.docker.com/mcp/catalog/v2/catalog.yaml`
- **Catalog Management System**: Full CRUD operations exist but are hidden and unused by gateway runtime
- **Multi-Catalog Gateway**: Infrastructure exists but defaults to Docker-only catalog
- **Docker Desktop**: Expects explicit catalog requests, doesn't auto-discover user catalogs

### Problem Statement
Users can create and manage custom catalogs using hidden commands (`create`, `add`, `import`, `fork`, `rm`), but the gateway runtime ignores these catalogs and only uses the hardcoded Docker catalog. This forces users to manually specify catalog paths via CLI flags for every gateway run.

## Goals

### Primary Goals
1. **Enable User-Managed Catalogs**: Allow users to create custom catalogs that automatically work with the gateway
2. **Maintain Docker Desktop Compatibility**: Ensure existing Docker Desktop integration remains unchanged
3. **Progressive Enhancement**: Provide opt-in feature activation with safe defaults
4. **Unified User Experience**: Eliminate the need for manual `--catalog` flags in typical usage

### Secondary Goals
1. **Discoverability**: Make catalog management commands visible to users
2. **Clear Precedence**: Define predictable catalog merge behavior when servers conflict
3. **Comprehensive Logging**: Provide visibility into catalog loading and conflicts
4. **Container Support**: Ensure feature works in containerized deployments

## User Stories

### US1: Local Development Workflow
**As a developer**, I want to create a custom catalog for my development MCP servers so that I can easily switch between development and production gateway configurations.

```bash
# Setup once
docker mcp feature enable configured-catalogs
docker mcp catalog create dev-servers
docker mcp catalog add dev-servers local-db ./local-db-server.yaml
docker mcp catalog add dev-servers test-api ./test-api-server.yaml

# Daily usage - custom servers automatically available
docker mcp gateway run --use-configured-catalogs
docker mcp tools enable local-db-query
```

### US2: Team Shared Catalogs
**As a team lead**, I want to import our team's shared catalog so that all team members have access to our custom MCP servers without manual configuration.

```bash
# One-time setup
docker mcp feature enable configured-catalogs
docker mcp catalog import https://internal.company.com/mcp/team-catalog.yaml

# Ongoing usage - team servers automatically available
docker mcp gateway run --use-configured-catalogs
```

### US3: Production Server Management
**As a DevOps engineer**, I want to combine official Docker servers with our production-specific servers in a single gateway instance.

```bash
# Production setup
docker mcp catalog create prod-monitoring  
docker mcp catalog add prod-monitoring datadog ./datadog-mcp.yaml
docker mcp catalog add prod-monitoring pagerduty ./pagerduty-mcp.yaml

# Gateway includes Docker + production servers automatically
docker mcp gateway run --use-configured-catalogs
```

### US4: Catalog Sharing and Backup
**As a team member**, I want to export my custom catalog configuration so I can share it with teammates or backup my configuration.

```bash
# Export custom catalog to share with team
docker mcp catalog export my-servers ./team-servers.yaml

# Team member imports the shared catalog
docker mcp catalog import ./team-servers.yaml

# Cannot export Docker's official catalog (protected)
docker mcp catalog export docker-mcp ./backup.yaml
# Error: Cannot export official Docker catalog 'docker-mcp'
```

### US5: Quick Start with Examples
**As a new user**, I want to easily understand the catalog format and get started with custom catalogs by having examples based on Docker's official servers.

```bash
# Create a starter catalog file with Docker Hub and Docker CLI as examples
docker mcp catalog bootstrap ./my-starter-catalog.yaml

# File contains properly formatted Docker and DockerHub entries as reference
# User can modify file to add their own servers
# Then import or use as source for other commands

docker mcp catalog import ./my-starter-catalog.yaml
# OR
docker mcp catalog add existing-catalog my-server ./my-starter-catalog.yaml
```

### US6: Docker Desktop Compatibility
**As a Docker Desktop user**, I want the existing catalog functionality to continue working without any changes to my workflow.

```bash
# This continues to work exactly as before - no user catalogs included
docker mcp gateway run

# Docker Desktop continues to request specific catalogs explicitly
# (Internal: docker mcp catalog show docker-mcp --format=json)
```

## Technical Requirements

### Functional Requirements

#### FR1: Feature Flag System
- **Feature Name**: `configured-catalogs`
- **Storage**: `~/.docker/config.json` in `features` object
- **Activation**: `docker mcp feature enable configured-catalogs`
- **Validation**: Gateway commands validate feature enablement before using configured catalogs

#### FR2: Gateway Integration  
- **New Flag**: `--use-configured-catalogs` for `docker mcp gateway run`
- **Behavior**: When enabled, includes user-managed catalogs from `~/.docker/mcp/catalog.json`
- **Fallback**: If configuration loading fails, continues with Docker catalog only
- **Logging**: Comprehensive logging of catalog loading and server conflicts

#### FR3: Catalog Precedence Order
When multiple catalogs contain the same server name, **last loaded wins**:

1. **Built-in Gateway catalogs** (future expansion point)
2. **Docker official catalog** (`docker-mcp.yaml`) 
3. **User configured catalogs** (from catalog management system) *[if `--use-configured-catalogs`]*
4. **CLI-specified catalogs** (`--catalog` and `--additional-catalog` flags)

#### FR4: Command Visibility
- **Unhide Commands**: Remove `Hidden: true` from catalog CRUD commands
- **New Export Command**: Add `export <catalog-name> [output-file]` command for user-managed catalogs
- **Documentation**: Update command help text to reference feature flag requirement
- **Discovery**: `docker mcp catalog --help` shows all available commands

#### FR5: Configuration Access
- **Plugin Mode**: Full access to `dockerCli.ConfigFile().Features`
- **Standalone Mode**: Same access pattern via Docker CLI infrastructure  
- **Container Mode**: Graceful degradation when config not mounted
- **Error Handling**: Clear messages when config inaccessible

### Non-Functional Requirements

#### NFR1: Backward Compatibility
- **Docker Desktop**: Zero changes to existing behavior when feature not enabled
- **CLI Compatibility**: All existing flags and commands work identically
- **Default Behavior**: `docker mcp gateway run` remains unchanged
- **Migration**: No existing configurations require updates

#### NFR2: Performance
- **Catalog Loading**: Minimal overhead when loading multiple catalogs
- **Conflict Resolution**: Efficient server deduplication across catalogs  
- **Startup Time**: Feature flag validation adds <50ms to command startup
- **Memory Usage**: Catalog merging uses reasonable memory footprint

#### NFR3: Security
- **Feature Isolation**: Disabled feature cannot access user catalogs
- **Config Validation**: Validate catalog URLs and file paths for safety
- **Access Control**: Respect Docker configuration directory permissions
- **Container Safety**: No privilege escalation in container mode

#### NFR4: Usability
- **Error Messages**: Clear, actionable error messages with exact commands to run  
- **Documentation**: Feature flag requirement clearly documented
- **Discoverability**: Users can discover catalog management without external documentation
- **Logging**: Gateway startup logs indicate which catalogs are active

## Technical Design

### Architecture Overview

```
Docker Desktop (unchanged)
├── docker mcp catalog show docker-mcp --format=json
└── docker mcp gateway run (Docker catalog only)

Manual CLI Usage (new capability)
├── docker mcp feature enable configured-catalogs  
├── docker mcp catalog create/add/import (now visible)
├── docker mcp gateway run --use-configured-catalogs
└── Automatic: Docker + User catalogs merged
```

### Implementation Components

#### 1. Feature Management Commands

**New Commands**:
```bash
docker mcp feature enable configured-catalogs   # Set features.configured-catalogs=enabled
docker mcp feature disable configured-catalogs  # Set features.configured-catalogs=disabled  
docker mcp feature list                         # Show all feature flags and status
```

**Implementation**: New `cmd/docker-mcp/commands/feature.go`

#### 2. Bootstrap Command (New)

**New Command**: `docker mcp catalog bootstrap <output-file-path>`

**Purpose**: Create a starter catalog file with Docker and Docker Hub server entries as examples, making it easy for users to understand the catalog format and quickly get started with custom catalogs.

**Behavior**:
- Extracts `dockerhub` and `docker` server entries from live Docker catalog
- Creates a properly formatted YAML catalog file at specified path
- File is standalone (not automatically imported) - ready for user modification
- Provides real working examples of catalog server definitions

**Usage Examples**:
```bash
# Create starter catalog with Docker examples
docker mcp catalog bootstrap ./my-starter-catalog.yaml

# Output file contains:
# registry:
#   dockerhub:
#     description: "Docker Hub official MCP server."
#     title: "Docker Hub"
#     image: "mcp/dockerhub@sha256:..."
#     tools: [...]
#     # ... complete server definition
#   docker:
#     description: "Use the Docker CLI."
#     title: "Docker"  
#     type: "poci"
#     # ... complete server definition

# User can then modify and import
docker mcp catalog import ./my-starter-catalog.yaml

# Or use as source for copying specific servers
docker mcp catalog add my-catalog docker-hub ./my-starter-catalog.yaml
```

**Implementation Strategy**:
1. **Config Loading**: Call `ReadConfigWithDefaultCatalog(ctx)` to load Docker catalog config
2. **YAML Reading**: Call `ReadCatalogFile("docker-mcp")` to get raw catalog YAML
3. **Struct Parsing**: Unmarshal YAML to `Registry` struct for Go data access
4. **Server Extraction**: Extract `registry.Registry["dockerhub"]` and `registry.Registry["docker"]` entries
5. **Bootstrap Generation**: Create new `Registry` struct with only extracted servers
6. **YAML Output**: Marshal to YAML and write standalone catalog file

**Error Handling**:
- Validate output path is writable
- Handle Docker catalog access failures gracefully
- Provide overwrite protection/confirmation for existing files

**Implementation**: New `cmd/docker-mcp/commands/bootstrap.go` and `cmd/docker-mcp/catalog/bootstrap.go`

#### 3. Gateway Command Enhancement

**Modified Command**: `docker mcp gateway run`

**New Flag**: `--use-configured-catalogs`

**Validation Logic**:
```go
func validateConfiguredCatalogsFeature(dockerCli command.Cli, useConfigured bool) error {
    if !useConfigured {
        return nil // No validation when feature not requested
    }
    
    featuresMap := dockerCli.ConfigFile().Features
    if v, ok := featuresMap["configured-catalogs"]; ok {
        if enabled, err := strconv.ParseBool(v); err == nil && enabled {
            return nil // Feature enabled
        }
    }
    
    return fmt.Errorf(`configured catalogs feature is not enabled.

To enable this experimental feature, run:
  docker mcp feature enable configured-catalogs

This feature allows the gateway to automatically include user-managed catalogs
alongside the default Docker catalog.`)
}
```

#### 3. Catalog Loading Enhancement

**Modified Function**: `cmd/docker-mcp/catalog/catalog.go:Get()`

**Current**:
```go
func Get(ctx context.Context) (Catalog, error) {
    return ReadFrom(ctx, []string{"docker-mcp.yaml"})
}
```

**New**:
```go  
func Get(ctx context.Context, useConfigured bool, additionalCatalogs []string) (Catalog, error) {
    var catalogSources []string
    
    // 1. Future: Built-in catalogs
    // catalogSources = append(catalogSources, getBuiltinCatalogs()...)
    
    // 2. Docker official catalog (always included)
    catalogSources = append(catalogSources, "docker-mcp.yaml")
    
    // 3. User configured catalogs (only if feature enabled)
    if useConfigured {
        configuredCatalogs, err := getConfiguredCatalogs()
        if err != nil {
            log.Printf("Warning: failed to load configured catalogs: %v", err)
        } else {
            catalogSources = append(catalogSources, configuredCatalogs...)
        }
    }
    
    // 4. CLI-specified additional catalogs  
    catalogSources = append(catalogSources, additionalCatalogs...)
    
    return ReadFrom(ctx, catalogSources)
}

func getConfiguredCatalogs() ([]string, error) {
    cfg, err := catalogConfig.ReadConfig() // Read ~/.docker/mcp/catalog.json
    if err != nil {
        return nil, err
    }
    
    var catalogFiles []string
    for name := range cfg.Catalogs {
        catalogFiles = append(catalogFiles, name + ".yaml")
    }
    return catalogFiles, nil  
}
```

#### 4. Enhanced Conflict Resolution

**Modified Function**: `cmd/docker-mcp/catalog/catalog.go:ReadFrom()`

**Enhanced Logging**:
```go
func ReadFrom(ctx context.Context, fileOrURLs []string) (Catalog, error) {
    mergedServers := map[string]Server{}
    catalogSources := make(map[string]string) // server -> source catalog

    log.Printf("Loading %d catalogs in precedence order", len(fileOrURLs))
    
    for i, fileOrURL := range fileOrURLs {
        log.Printf("Loading catalog %d/%d: %s", i+1, len(fileOrURLs), fileOrURL)
        
        servers, err := readMCPServers(ctx, fileOrURL)
        if err != nil {
            log.Printf("Warning: failed to load catalog '%s': %v", fileOrURL, err)
            continue
        }

        for serverName, server := range servers {
            if existingSource, exists := catalogSources[serverName]; exists {
                log.Printf("SERVER OVERRIDE: '%s' from '%s' replaces version from '%s'", 
                    serverName, fileOrURL, existingSource)
            } else {
                log.Printf("SERVER ADDED: '%s' from '%s'", serverName, fileOrURL)
            }
            
            mergedServers[serverName] = server
            catalogSources[serverName] = fileOrURL
        }
    }

    log.Printf("Final catalog contains %d servers from %d catalogs", 
        len(mergedServers), len(fileOrURLs))

    return Catalog{Servers: mergedServers}, nil
}
```

#### 5. Export Command Implementation

**New Command**: `docker mcp catalog export <catalog-name> [output-file]`

**Purpose**: Export user-managed catalogs to files for sharing, backup, or migration.

**Implementation**: New `cmd/docker-mcp/catalog/export.go`

**Key Features**:
```go
func exportCatalog(ctx context.Context, catalogName, outputFile string) error {
    // 1. Validate catalog exists and is user-managed
    if catalogName == "docker-mcp" || catalogName == DockerCatalogName {
        return fmt.Errorf("cannot export official Docker catalog '%s'", catalogName)
    }
    
    // 2. Read catalog from ~/.docker/mcp/catalogs/{catalogName}.yaml
    catalogPath, err := catalogConfig.FilePath(fmt.Sprintf("catalogs/%s.yaml", catalogName))
    if err != nil {
        return err
    }
    
    catalogData, err := os.ReadFile(catalogPath)
    if err != nil {
        return fmt.Errorf("catalog '%s' not found", catalogName)
    }
    
    // 3. Default output file if not specified
    if outputFile == "" {
        outputFile = fmt.Sprintf("./%s.yaml", catalogName)
    }
    
    // 4. Write to output file
    return os.WriteFile(outputFile, catalogData, 0644)
}
```

**Protection Logic**:
- **Official Catalog Protection**: Prevent export of `docker-mcp` catalog
- **User Catalog Only**: Only export catalogs that exist in `~/.docker/mcp/catalogs/`
- **File Validation**: Ensure target catalog exists before attempting export

**Usage Examples**:
```bash
# Export to default filename (my-servers.yaml)
docker mcp catalog export my-servers

# Export to specific file
docker mcp catalog export my-servers ./shared/team-catalog.yaml

# Export for backup
docker mcp catalog export prod-monitoring ./backups/prod-$(date +%Y%m%d).yaml
```

#### 6. Command Visibility Updates

**Modified Files**: All catalog command files in `cmd/docker-mcp/catalog/`

**Changes**: Remove `Hidden: true` from:
- `import.go` - Import catalogs from URLs/files
- `export.go` - Export user catalogs to files (new command)
- `create.go` - Create empty local catalogs  
- `add.go` - Add servers to catalogs
- `fork.go` - Duplicate existing catalogs
- `rm.go` - Remove catalogs

**Enhanced Help Text**: Reference feature flag requirement in command descriptions.

### Container Mode Support

**Volume Mount Requirement**:
```bash
# For feature flags to work in container mode
docker run -v ~/.docker:/root/.docker docker/mcp-gateway gateway run --use-configured-catalogs

# Or mount just the config
docker run -v ~/.docker/config.json:/root/.docker/config.json docker/mcp-gateway gateway run --use-configured-catalogs
```

**Enhanced Error Handling**:
```go
func validateConfiguredCatalogsFeature(dockerCli command.Cli, useConfigured bool) error {
    if !useConfigured {
        return nil
    }
    
    configFile := dockerCli.ConfigFile()
    if configFile == nil {
        return fmt.Errorf(`Docker configuration not accessible.

If running in container, mount Docker config:
  -v ~/.docker:/root/.docker
  
Or mount just the config file:  
  -v ~/.docker/config.json:/root/.docker/config.json`)
    }
    
    // Continue with feature validation...
}
```

## Testing Strategy

### Unit Tests

#### Test Coverage Areas
1. **Feature Flag Validation**: Test enabled/disabled/missing scenarios
2. **Catalog Loading**: Test precedence order and conflict resolution  
3. **Error Handling**: Test config inaccessible, malformed catalogs
4. **Container Mode**: Test behavior without volume mounts

#### Key Test Cases
```go
func TestCatalogPrecedenceOrder(t *testing.T) {
    // Test that CLI catalogs override configured catalogs override Docker catalog
}

func TestFeatureFlagValidation(t *testing.T) {
    // Test feature flag validation in various states
}

func TestContainerModeGracefulDegradation(t *testing.T) {
    // Test behavior when Docker config is inaccessible
}
```

### Integration Tests

#### Test Scenarios
1. **End-to-End Workflow**: Create catalog → Add server → Enable feature → Run gateway → Verify server availability
2. **Docker Desktop Compatibility**: Verify existing workflows unchanged
3. **Multi-Catalog Conflicts**: Test server override behavior across catalogs
4. **Feature Flag Lifecycle**: Enable → Use → Disable → Verify disabled

#### Test Environment
- **Docker Desktop**: Verify no regression in existing behavior
- **Standalone Binary**: Verify feature works in standalone mode  
- **Container**: Verify graceful degradation without volume mounts

## Migration Strategy

### Phase 1: Implementation (No Breaking Changes)
1. **Add Feature Management**: Implement `docker mcp feature` commands
2. **Enhance Gateway**: Add `--use-configured-catalogs` flag with validation
3. **Update Catalog Loading**: Implement precedence-based catalog merging
4. **Unhide Commands**: Make catalog CRUD commands visible
5. **Comprehensive Testing**: Unit and integration test coverage

### Phase 2: Documentation and User Communication
1. **User Documentation**: Update CLI reference and tutorials
2. **Migration Guide**: Document feature enablement and workflows
3. **Release Notes**: Highlight new capability and backward compatibility
4. **Community Communication**: Blog post explaining enhanced catalog management

### Phase 3: Future Enhancements (Optional)
1. **Docker Desktop Integration**: UI for managing custom catalogs
2. **Catalog Signing**: Implement signature verification for remote catalogs
3. **Default Feature**: Consider making configured catalogs default in future major version
4. **Well-Known Catalogs**: Add community catalog aliases

## Risk Assessment

### High-Impact Risks

#### Risk: Docker Desktop Regression
- **Probability**: Low
- **Impact**: High
- **Mitigation**: Extensive testing, feature flag gating, unchanged default behavior

#### Risk: Configuration Corruption  
- **Probability**: Low
- **Impact**: Medium
- **Mitigation**: Atomic config updates, backup/restore commands, validation

### Medium-Impact Risks

#### Risk: Performance Degradation
- **Probability**: Low  
- **Impact**: Medium
- **Mitigation**: Efficient catalog loading, caching, performance testing

#### Risk: User Confusion
- **Probability**: Medium
- **Impact**: Low
- **Mitigation**: Clear error messages, comprehensive documentation, progressive disclosure

### Low-Impact Risks

#### Risk: Container Mode Limitations
- **Probability**: Medium
- **Impact**: Low  
- **Mitigation**: Clear documentation, graceful degradation, helpful error messages

## Success Metrics

### User Adoption
- **Feature Enablement**: Track `configured-catalogs` feature activation
- **Command Usage**: Monitor usage of previously hidden catalog commands
- **Gateway Usage**: Track `--use-configured-catalogs` flag adoption

### Technical Metrics  
- **Performance**: Gateway startup time with multiple catalogs
- **Reliability**: Error rates when loading configured catalogs
- **Compatibility**: Docker Desktop integration regression testing results

### User Experience
- **Support Requests**: Reduction in catalog-related support questions
- **Documentation Engagement**: Usage of new catalog management documentation
- **Community Feedback**: User satisfaction with enhanced catalog workflow

## Open Questions

1. **Default Behavior**: Should configured catalogs eventually become the default (breaking change)?
2. **Catalog Validation**: What level of validation should be applied to imported catalogs?
3. **Performance Optimization**: Should catalog loading be cached between gateway runs?
4. **Docker Desktop UI**: When should Docker Desktop add catalog management UI?

## Conclusion

This feature specification provides a comprehensive, backward-compatible enhancement to MCP Gateway catalog management. By using Docker CLI's existing feature flag system and maintaining strict compatibility with Docker Desktop, users gain powerful catalog management capabilities while ensuring existing workflows remain unchanged.

The implementation leverages existing infrastructure (Jim Clark's multi-catalog support, David Gageot's catalog management system) to provide a complete solution with minimal risk and maximum user value.