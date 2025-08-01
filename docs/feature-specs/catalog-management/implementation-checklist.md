# User-Managed Catalogs Implementation Checklist

## Project Status: ✅ PHASE 1 & 2 COMPLETE - PRODUCTION READY

**Last Updated**: August 1, 2025  
**Feature Spec**: [feature-spec.md](./feature-spec.md)  
**Investigation Notes**: `/Users/masegraye/dev/docker/id-writing/scratch/mcp-gateway-investigation.md`

### 🎉 Implementation Summary
All Phase 1 & 2 implementation work has been **completed and tested**:
- ✅ **Feature Management System**: Full TDD implementation with 8 test cases
- ✅ **Gateway Enhancement**: Complete flag integration with 5 test cases  
- ✅ **Catalog Loading**: Multi-catalog support with 6 test cases
- ✅ **Export Command**: New functionality with 4 test cases
- ✅ **Bootstrap Command**: New quick-start functionality with 4 test cases
- ✅ **Command Visibility**: All CRUD commands now user-accessible
- ✅ **Container Test Suite**: All tests pass in Docker container environment
- ✅ **Binary Build**: Successful compilation and CLI plugin installation
- ✅ **End-to-End Workflow**: Complete bootstrap → add → export validation

## Development Workflow & TDD Instructions

### Essential Commands for Development

All commands should be run from `/Users/masegraye/dev/docker/workspaces/catalog-management/mcp-gateway/`:

```bash
# Scoped Test-Driven Development Cycle
go test ./cmd/docker-mcp/commands           # Test specific package (fastest)
go test ./cmd/docker-mcp/...               # Test cmd tree (medium scope)
make test                                   # Test entire system (broadest scope)
make docker-mcp                            # Build and install binary (only after tests pass)
make lint-darwin                           # Run linting (only when implementation complete)

# Manual testing commands
./dist/docker-mcp --help                    # Verify binary works
./dist/docker-mcp catalog --help           # Check catalog commands
./dist/docker-mcp gateway run --help       # Check gateway flags
```

### TDD Development Process

**CRITICAL**: Follow this exact workflow with progressively broader scope:

#### 1. Red Phase - Write Failing Tests First
```bash
# Test just the specific package you're working on
go test ./cmd/docker-mcp/commands    # Example: testing feature commands
# Should show failing tests for new functionality
```

#### 2. Green Phase - Make Tests Pass
```bash
# Write minimal implementation to make tests pass
go test ./cmd/docker-mcp/commands    # Test same package again
# Should show all tests passing in that package
```

#### 3. Package Integration Test
```bash
# Test all related packages together
go test ./cmd/docker-mcp/...         # Test entire cmd tree
# Should show all tests passing across packages
```

#### 4. Full System Test
```bash
# Test the entire system
make test
# Should show all tests passing system-wide
```

#### 5. Build Phase - Verify Compilation
```bash
# Only build after all tests pass
make docker-mcp
# Should build successfully and install to ~/.docker/cli-plugins/docker-mcp
```

#### 6. Manual Verification
```bash
# Test the actual CLI functionality
./dist/docker-mcp [your-new-command] --help
# Verify your changes work as expected
```

#### 7. Refactor Phase (Optional)
```bash
# Improve code while keeping tests green
go test ./path/to/package    # Test specific package after changes
```

#### 8. Lint Phase (End of Implementation Only)
```bash
# Only run linting when feature is complete
make lint-darwin
# Should show 0 issues
```

### Test Categories & Strategy

#### Unit Tests (Primary Focus)
- **Location**: Tests should be in `*_test.go` files alongside implementation
- **Speed**: Fast (< 1 second per test file)
- **Focus**: Individual function/method behavior
- **Run Command**: `make test`

#### Integration Tests (Secondary)
- **Location**: `cmd/docker-mcp/integration_test.go` and similar
- **Speed**: Slower (requires Docker daemon)
- **Focus**: End-to-end command workflows
- **Note**: Some are skipped in normal `make test` runs

#### Manual Tests (Verification)
- **Purpose**: Verify CLI behavior matches expectations
- **When**: After successful build, before marking tasks complete
- **Focus**: User experience and error messages

### Implementation Guidelines

1. **Test First**: Always write tests before implementation
2. **Small Steps**: Implement one small piece at a time
3. **Green Tests**: Never commit with failing tests
4. **Build Validation**: Only build after tests pass
5. **Manual Check**: Always verify CLI behavior manually
6. **Lint Last**: Only run linting when implementation is complete

### Error Handling Strategy

#### Test Failures
```bash
make test
# If tests fail, fix the code, don't skip tests
```

#### Build Failures
```bash
make docker-mcp
# If build fails, check compilation errors and fix
# Don't proceed to manual testing until build succeeds
```

#### Lint Failures (End of Development)
```bash
make lint-darwin
# Fix all linting issues before marking feature complete
```

### Baseline Verification ✅

The following baseline tests have been verified to work:
- `make docker-mcp` - Builds successfully 
- `make test` - All existing tests pass
- `make lint-darwin` - 0 linting issues
- CLI functionality verified working

**Ready to begin TDD implementation following the above workflow.**

## 🧪 Test Strategy Overview

### Required Test Files for TDD Implementation

Each implementation section below specifies **TEST FIRST** requirements. These test files must be created with failing tests before writing implementation code:

| Component | Test File | Purpose |
|-----------|-----------|---------|
| Feature Management | `cmd/docker-mcp/commands/feature_test.go` | Test feature enable/disable/list commands |
| Gateway Enhancement | `cmd/docker-mcp/commands/gateway_test.go` | Test --use-configured-catalogs flag validation |
| Catalog Loading | `cmd/docker-mcp/internal/catalog/catalog_test.go` | Test catalog precedence and loading logic |
| Export Command | `cmd/docker-mcp/catalog/export_test.go` | Test export functionality and protection |
| Command Visibility | `cmd/docker-mcp/catalog/*_test.go` | Test unhidden commands appear in help |

### Test-First Workflow Reminder

```bash
# 1. Write failing tests first
go test ./cmd/docker-mcp/commands -v  # Should show failing tests

# 2. Write minimal implementation  
go test ./cmd/docker-mcp/commands -v  # Should show passing tests

# 3. Test broader scope
go test ./cmd/docker-mcp/... -v      # Should show all integration tests pass

# 4. Test full system
make test                            # Should show all system tests pass

# 5. Build and verify
make docker-mcp                      # Should build successfully
```

## Quick Context for Claude Code Sessions

### What This Feature Does
Enable users to create and manage custom MCP server catalogs that automatically work with the gateway runtime, while maintaining backward compatibility with Docker Desktop through feature flag gating.

### Current Architecture Status
- ✅ **Catalog CRUD System**: Fully implemented by David Gageot (June-July 2025)
- ✅ **Multi-Catalog Gateway**: Implemented by Jim Clark (July 21, 2025) 
- ✅ **Infrastructure**: All underlying systems exist and work
- ❌ **Integration Gap**: Gateway runtime ignores catalog management system
- ❌ **User Discovery**: Catalog management commands are hidden

### Implementation Strategy
**Feature Flag Approach**: Use Docker CLI's existing `features` config system to gate new functionality, ensuring Docker Desktop compatibility.

## Implementation Checklist

### Phase 1: Core Implementation ✅ COMPLETED

#### 1.1 Feature Management System ✅ COMPLETED

**🧪 TEST FIRST**: `cmd/docker-mcp/commands/feature_test.go` ✅ COMPLETED
- [x] **Write tests for feature command structure**
  ```go
  // Test cases implemented:
  func TestFeatureEnableCommand(t *testing.T)     // ✅ Test enabling configured-catalogs
  func TestFeatureDisableCommand(t *testing.T)    // ✅ Test disabling configured-catalogs  
  func TestFeatureListCommand(t *testing.T)       // ✅ Test listing all features and status
  func TestFeatureInvalidFeature(t *testing.T)    // ✅ Test error for unknown feature names
  ```

- [x] **Create feature command structure** ✅ COMPLETED
  - [x] File: `cmd/docker-mcp/commands/feature.go`
  - [x] Commands: `enable <feature>`, `disable <feature>`, `list`
  - [x] Target: `~/.docker/config.json` → `features.configured-catalogs`

**🧪 TEST FIRST**: `cmd/docker-mcp/commands/feature_test.go` (validation utilities) ✅ COMPLETED
- [x] **Write tests for feature validation utilities**
  ```go
  // Test cases implemented:
  func TestIsFeatureEnabledTrue(t *testing.T)       // ✅ Test when feature is enabled
  func TestIsFeatureEnabledFalse(t *testing.T)      // ✅ Test when feature is disabled
  func TestIsFeatureEnabledMissing(t *testing.T)    // ✅ Test when config missing
  func TestIsFeatureEnabledCorrupt(t *testing.T)    // ✅ Test when config corrupted
  ```

- [x] **Feature validation utilities** ✅ COMPLETED
  - [x] Function: `isFeatureEnabled(dockerCli command.Cli, feature string) bool`
  - [x] Handle missing config file gracefully
  - [x] Support container mode detection

- [x] **Integration with root command** ✅ COMPLETED
  - [x] Add feature command to `cmd/docker-mcp/commands/root.go`
  - [x] Ensure proper dockerCli context passing

#### 1.2 Gateway Command Enhancement ✅ COMPLETED

**🧪 TEST FIRST**: `cmd/docker-mcp/commands/gateway_test.go` ✅ COMPLETED
- [x] **Write tests for gateway flag validation**
  ```go
  // Test cases implemented:
  func TestGatewayUseConfiguredCatalogsEnabled(t *testing.T)   // ✅ Test flag works when feature enabled
  func TestGatewayUseConfiguredCatalogsDisabled(t *testing.T)  // ✅ Test flag fails when feature disabled
  func TestGatewayFeatureFlagErrorMessage(t *testing.T)        // ✅ Test error message clarity
  func TestGatewayContainerModeDetection(t *testing.T)         // ✅ Test container mode handling
  func TestGatewayNoValidationWhenFlagNotUsed(t *testing.T)    // ✅ Test no validation when flag not used
  ```

- [x] **Add --use-configured-catalogs flag** ✅ COMPLETED
  - [x] File: `cmd/docker-mcp/commands/gateway.go`
  - [x] Flag: `--use-configured-catalogs` (boolean)
  - [x] Validation: Check feature flag before allowing flag usage

- [x] **Feature validation integration** ✅ COMPLETED
  - [x] PreRunE validation for feature flag requirement
  - [x] Clear error messages with exact enable command
  - [x] Container mode detection and helpful errors

- [x] **Pass flag to catalog system** ✅ COMPLETED
  - [x] Update `catalog.Get()` call site
  - [x] Pass useConfigured boolean parameter

#### 1.3 Catalog Loading Enhancement ✅ COMPLETED

**🧪 TEST FIRST**: `cmd/docker-mcp/internal/catalog/catalog_test.go` ✅ COMPLETED
- [x] **Write tests for catalog loading logic**
  ```go
  // Test cases implemented:
  func TestCatalogGetWithConfigured(t *testing.T)         // ✅ Test loading configured catalogs
  func TestCatalogGetWithoutConfigured(t *testing.T)      // ✅ Test default behavior unchanged
  func TestGetConfiguredCatalogsSuccess(t *testing.T)     // ✅ Test reading catalog.json
  func TestGetConfiguredCatalogsMissing(t *testing.T)     // ✅ Test missing catalog.json
  func TestGetConfiguredCatalogsCorrupt(t *testing.T)     // ✅ Test corrupted catalog.json
  func TestCatalogPrecedenceOrder(t *testing.T)           // ✅ Test Docker → Configured → CLI order
  ```

- [x] **Update catalog.Get() signature** ✅ COMPLETED  
  - [x] File: `cmd/docker-mcp/internal/catalog/catalog.go`
  - [x] New function: `GetWithOptions(ctx context.Context, useConfigured bool, additionalCatalogs []string) (Catalog, error)`
  - [x] Backward compatibility: Keep current `Get()` for existing callers

- [x] **Implement getConfiguredCatalogs()** ✅ COMPLETED
  - [x] Read from `~/.docker/mcp/catalog.json`
  - [x] Return list of catalog file names
  - [x] Handle missing/corrupted catalog registry gracefully

- [x] **Implement catalog precedence logic** ✅ COMPLETED
  - [x] Order: Docker → Configured → CLI-specified
  - [x] Use existing `ReadFrom()` function with ordered list
  - [x] Comprehensive logging of loading process with warnings

#### 1.4 Enhanced Conflict Resolution & Logging ✅ COMPLETED

**🧪 TEST FIRST**: `cmd/docker-mcp/internal/catalog/catalog_test.go` (conflict resolution) ✅ COMPLETED
- [x] **Write tests for conflict resolution**
  ```go
  // Test cases implemented:
  func TestCatalogPrecedenceOrder(t *testing.T)     // ✅ Test server-level precedence (architecturally correct)
  // Additional conflict resolution validated through existing ReadFrom() behavior
  ```

- [x] **Validate ReadFrom() logging** ✅ COMPLETED
  - [x] File: `cmd/docker-mcp/internal/catalog/catalog.go`
  - [x] Existing warning messages for server conflicts confirmed working
  - [x] Server-level precedence correctly implemented

- [x] **Server conflict handling** ✅ COMPLETED
  - [x] Confirmed server-level replacement prevents tool mixing issues
  - [x] "Last wins" precedence behavior validated as architecturally correct
  - [x] Gateway tool flattening works properly with resolved servers

#### 1.5 Export Command Implementation ✅ COMPLETED

**🧪 TEST FIRST**: `cmd/docker-mcp/commands/export_test.go` ✅ COMPLETED
- [x] **Write tests for export command**
  ```go
  // Test cases implemented:
  func TestExportCatalogCommand(t *testing.T)           // ✅ Test successful export
  func TestExportDockerCatalogShouldFail(t *testing.T)  // ✅ Test docker-mcp protection
  func TestExportNonExistentCatalog(t *testing.T)       // ✅ Test missing catalog error
  func TestExportInvalidOutputPath(t *testing.T)        // ✅ Test file permission/path errors
  ```

- [x] **Create export command** ✅ COMPLETED
  - [x] File: `cmd/docker-mcp/catalog/export.go` (new)
  - [x] Command: `export <catalog-name> <file-path>`
  - [x] Protection: Prevent export of `docker-mcp` official catalog

- [x] **Export functionality** ✅ COMPLETED
  - [x] Read catalog from `~/.docker/mcp/catalogs/{name}.yaml`
  - [x] Validate catalog exists and is user-managed
  - [x] Support for custom output file path

- [x] **Error handling** ✅ COMPLETED
  - [x] Clear error for attempting to export official Docker catalog
  - [x] Helpful error when catalog doesn't exist
  - [x] File system permission error handling

#### 1.6 Command Visibility Updates ✅ COMPLETED

**🧪 TEST FIRST**: Command visibility validated through CLI help output ✅ COMPLETED
- [x] **Manual validation of command visibility**
  ```bash
  # All commands now visible in docker mcp catalog --help:
  # ✅ add         Add a server to your catalog
  # ✅ create      Create a new catalog
  # ✅ export      Export a configured catalog to a file
  # ✅ fork        Fork a catalog
  # ✅ import      Import a catalog
  # ✅ rm          Remove a catalog
  ```

- [x] **Unhide catalog CRUD commands** ✅ COMPLETED
  - [x] Files: `cmd/docker-mcp/commands/catalog.go`  
  - [x] Removed `Hidden: true` from all command definitions
  - [x] All catalog management commands now user-visible

- [x] **Update command descriptions** ✅ COMPLETED
  - [x] Export command includes clear protection messaging
  - [x] Help text explains user-managed vs Docker catalog distinction

### Phase 2: Testing & Validation ✅ COMPLETED

#### 2.1 Unit Tests ✅ COMPLETED

- [x] **Feature flag validation tests** ✅ COMPLETED
  - [x] Test enabled/disabled/missing scenarios (4 test cases)
  - [x] Test Docker CLI integration
  - [x] Test container mode behavior

- [x] **Catalog loading tests** ✅ COMPLETED  
  - [x] Test precedence order with multiple catalogs (6 test cases)
  - [x] Test conflict resolution logging with warning messages
  - [x] Test graceful failure handling for missing/corrupt registry

- [x] **Export command tests** ✅ COMPLETED
  - [x] Test successful export of user catalogs
  - [x] Test prevention of Docker catalog export
  - [x] Test catalog not found scenarios
  - [x] Test file permission error handling

- [x] **Gateway integration tests** ✅ COMPLETED
  - [x] Test flag validation (5 test cases)
  - [x] Test feature flag gating with clear error messages
  - [x] Test container mode detection and guidance

#### 2.2 Integration Tests ✅ COMPLETED

- [x] **End-to-end workflow tests** ✅ COMPLETED
  - [x] All test suites pass in container environment
  - [x] Binary builds and installs successfully as Docker CLI plugin
  - [x] All commands visible and functional in CLI help

- [x] **Docker Desktop compatibility tests** ✅ COMPLETED
  - [x] Verified `docker mcp gateway run` behavior unchanged (backward compatibility)
  - [x] Verified existing catalog commands work as expected
  - [x] No regression in Docker Desktop workflow

- [x] **Container mode tests** ✅ COMPLETED
  - [x] Tests pass in Docker container environment (`make test`)
  - [x] Feature flag validation works in container mode
  - [x] Clear error messages for missing config scenarios

#### 2.3 Manual Testing Scenarios ✅ COMPLETED

- [x] **Development workflow testing** ✅ COMPLETED
  - [x] Feature enable/disable cycle verified working
  - [x] All catalog commands now visible in `docker mcp catalog --help`
  - [x] Export command protection working correctly
  - [x] Binary installation to `~/.docker/cli-plugins/docker-mcp` successful

- [x] **Multi-catalog testing** ✅ COMPLETED
  - [x] Server-level precedence correctly implemented and tested
  - [x] Warning logging for overlapping servers verified working
  - [x] Architecture validated as correct for tool flattening

### Phase 2: Bootstrap Command Implementation

#### 2.1 Bootstrap Command Design ✅ COMPLETED

**Command Name**: `docker mcp catalog bootstrap <output-file-path>`

**Purpose**: Create a starter catalog file with Docker and Docker Hub server entries as examples, making it easy for users to understand the catalog format and get started with custom catalogs.

**🧪 TEST FIRST**: `cmd/docker-mcp/commands/bootstrap_test.go` ✅ COMPLETED
- [x] **Write tests for bootstrap command**
  ```go
  // Test cases implemented:
  func TestBootstrapCatalogCommand(t *testing.T)        // ✅ Test successful bootstrap creation
  func TestBootstrapExistingFile(t *testing.T)          // ✅ Test overwrite protection/confirmation
  func TestBootstrapInvalidPath(t *testing.T)           // ✅ Test invalid output path handling
  func TestBootstrapDockerEntriesExtraction(t *testing.T) // ✅ Test Docker/DockerHub entry extraction
  ```

**Implementation Strategy**:
1. **Config loading**: Call `ReadConfigWithDefaultCatalog(ctx)` to load Docker catalog
2. **YAML reading**: Call `ReadCatalogFile("docker-mcp")` to get raw catalog YAML  
3. **Struct parsing**: Unmarshal to `Registry` struct for Go data access
4. **Server extraction**: Extract `registry.Registry["dockerhub"]` and `registry.Registry["docker"]`
5. **Bootstrap generation**: Create new `Registry` with only extracted servers  
6. **File creation**: Marshal to YAML and write standalone catalog file

**Expected User Workflow**:
```bash
# Create bootstrap catalog with Docker examples
docker mcp catalog bootstrap ./my-custom-catalog.yaml

# User modifies the file to add their own servers
# User can then import it
docker mcp catalog import ./my-custom-catalog.yaml

# Or use it as source for adding individual servers
docker mcp catalog add existing-catalog my-server ./my-custom-catalog.yaml
```

**Implementation Tasks**: ✅ ALL COMPLETED
- [x] **Create bootstrap command** ✅ COMPLETED
  - [x] File: `cmd/docker-mcp/commands/bootstrap.go` (new)
  - [x] Command: `bootstrap <output-file-path>`
  - [x] Internal calls to `ReadConfigWithDefaultCatalog()` and `ReadCatalogFile()`

- [x] **Bootstrap functionality** ✅ COMPLETED  
  - [x] File: `cmd/docker-mcp/catalog/bootstrap.go` (new)
  - [x] Extract Docker and DockerHub entries from live catalog using YAML parsing
  - [x] Generate properly formatted YAML catalog structure with extracted servers
  - [x] File overwrite protection with clear error messages

- [x] **Error handling** ✅ COMPLETED
  - [x] Validate output path is writable with directory creation
  - [x] Handle Docker catalog access failures gracefully
  - [x] Clear error messages for file conflicts and missing servers

- [x] **Integration with catalog command** ✅ COMPLETED
  - [x] Added bootstrap command to `cmd/docker-mcp/commands/catalog.go`
  - [x] Comprehensive help text and examples
  - [x] Full CLI integration and visibility

**End-to-End Validation**: ✅ COMPLETED
- [x] **Bootstrap file creation**: Successfully creates YAML with Docker and DockerHub servers
- [x] **Catalog add integration**: Bootstrap file works as source for `catalog add` command
- [x] **Server extraction**: Individual servers can be copied from bootstrap file to catalogs
- [x] **Export roundtrip**: Full workflow (bootstrap → add → export) validated
- [x] **Real-world usage**: Tested with actual Docker catalog data and server definitions

### Phase 3: Documentation & Polish

#### 3.1 Documentation Updates

- [ ] **CLI reference updates**
  - [ ] Update generated docs for newly visible commands
  - [ ] Document feature flag requirement
  - [ ] Document new bootstrap command workflow
  - [ ] Provide usage examples

- [ ] **User workflow documentation**
  - [ ] Development workflow example including bootstrap
  - [ ] Production setup guide
  - [ ] Container deployment instructions

#### 3.2 Error Handling Polish

- [ ] **Comprehensive error messages**
  - [ ] Feature not enabled → exact enable command
  - [ ] Container mode → volume mount instructions  
  - [ ] Catalog loading failures → helpful diagnostics

- [ ] **Logging improvements**
  - [ ] Structured logging for catalog operations
  - [ ] Clear startup messages about active catalogs
  - [ ] Performance timing for catalog loading

### Phase 4: Advanced Features (Future)

#### 4.1 Performance Optimizations
- [ ] **Catalog caching**
  - [ ] Cache loaded catalogs between gateway runs
  - [ ] Invalidate cache on catalog updates
  - [ ] Performance benchmarking

#### 4.2 Enhanced Validation
- [ ] **Catalog URL validation**
  - [ ] HTTPS requirement for remote catalogs
  - [ ] URL safety validation
  - [ ] Optional signature verification

#### 4.3 Docker Desktop Integration
- [ ] **UI for catalog management**
  - [ ] List configured catalogs in Docker Desktop
  - [ ] Enable/disable specific catalogs
  - [ ] Import catalogs via UI

## Progress Tracking

### Completed Analysis Work ✅
- [x] **Architecture Investigation**: Complete understanding of existing systems
- [x] **Git Blame Analysis**: Identified key contributors and implementation timeline  
- [x] **Docker Desktop Integration**: Understood compatibility requirements
- [x] **Binary Distribution Analysis**: Confirmed feature flag system works across all modes
- [x] **Feature Flag Research**: Identified Docker CLI `features` system as perfect solution
- [x] **Implementation Planning**: Complete technical design and user workflows
- [x] **Feature Specification**: Comprehensive spec document created
- [x] **Risk Assessment**: Identified and mitigated major risks

### Current Status
**✅ PHASE 1 & 2 IMPLEMENTATION COMPLETE - PRODUCTION READY**

All Phase 1 & 2 implementation, testing, and validation work is **complete**. The feature is ready for production use with:

- **27 comprehensive test cases** covering all functionality (Phase 1: 23 tests, Phase 2: 4 tests)
- **Full TDD implementation** with test-first development methodology
- **Container environment validation** - all tests pass in `make test`
- **End-to-End workflow validation** - complete bootstrap → add → export tested
- **Backward compatibility** - no changes to existing Docker Desktop workflows  
- **Production-ready binary** - builds and installs successfully as Docker CLI plugin

**Key Achievements**: 
- Users can now enable `configured-catalogs` feature and use custom catalogs alongside Docker's official catalog with full CLI management capabilities
- New users can quickly get started with the `bootstrap` command to understand catalog format and create starter files with real Docker server examples

## Key Implementation Notes

### Critical Success Factors
1. **Maintain Docker Desktop Compatibility**: Never change default behavior of `docker mcp gateway run`
2. **Feature Flag Gating**: All new functionality MUST be gated behind `configured-catalogs` feature
3. **Graceful Degradation**: System must work properly when config is inaccessible
4. **Clear User Communication**: Error messages must provide exact commands to fix issues

### Technical Constraints  
1. **Backward Compatibility**: All existing command signatures and behaviors preserved
2. **Container Support**: Feature must work in container mode with proper volume mounts
3. **Performance**: Catalog loading must not significantly impact gateway startup time
4. **Security**: No privilege escalation or unsafe file operations

### Architecture Decisions Made
1. **Feature Flag System**: Use Docker CLI's existing `features` config approach
2. **Catalog Precedence**: Last-loaded-wins for server conflicts (Docker → Configured → CLI)
3. **Command Structure**: Add `--use-configured-catalogs` flag rather than change defaults
4. **Error Handling**: Fail-safe approach where config problems disable feature rather than break gateway

## File Locations Reference

### Implementation Target Files
- **Feature Commands**: `cmd/docker-mcp/commands/feature.go` (new)
- **Export Command**: `cmd/docker-mcp/catalog/export.go` (new)
- **Gateway Enhancement**: `cmd/docker-mcp/commands/gateway.go` 
- **Catalog Loading**: `cmd/docker-mcp/catalog/catalog.go`
- **Command Visibility**: `cmd/docker-mcp/catalog/{import,export,create,add,fork,rm}.go`

### Key Reference Files  
- **Command Structure**: `cmd/docker-mcp/commands/root.go`
- **Docker CLI Integration**: `cmd/docker-mcp/main.go:33-35`
- **Config Pattern**: `cli/cli/config/configfile/file.go:120` (Features map)
- **Storage Pattern**: `cmd/docker-mcp/internal/config/readwrite.go`

### Investigation Documentation
- **Detailed Investigation**: `/Users/masegraye/dev/docker/id-writing/scratch/mcp-gateway-investigation.md`
- **Feature Specification**: `docs/feature-specs/catalog-management/feature-spec.md`

## Contact Information

**Primary Technical Contact**: Jim Clark (@slimslenderslacks)
- Implemented multi-catalog gateway support (July 21, 2025)
- Best person to consult for architecture questions

**Secondary Contact**: David Gageot  
- Implemented catalog management system (June-July 2025)
- Expert on catalog CRUD operations and storage patterns

---

## Quick Start for New Claude Code Sessions

1. **Read the feature spec**: Start with `feature-spec.md` for complete context
2. **Check current status**: Review completed items in this checklist
3. **Start implementation**: Begin with Phase 1.1 (Feature Management System)
4. **Test thoroughly**: Each component must work in plugin, standalone, and container modes  
5. **Maintain compatibility**: Docker Desktop behavior must never change

**Key Commands to Validate Success**:
```bash
# Phase 1: Configured catalogs workflow
docker mcp feature enable configured-catalogs
docker mcp catalog create my-servers  
docker mcp catalog add my-servers test-server ./server.yaml
docker mcp catalog export my-servers ./backup.yaml
docker mcp gateway run --use-configured-catalogs
# Gateway should start with both Docker servers AND test-server available

# Phase 2: Bootstrap workflow for new users
docker mcp catalog bootstrap ./starter-catalog.yaml
# Creates file with Docker and DockerHub server examples
docker mcp catalog create custom-servers
docker mcp catalog add custom-servers dockerhub ./starter-catalog.yaml
# Copies Docker Hub server from bootstrap file to custom catalog

# Validate export protection
docker mcp catalog export docker-mcp ./should-fail.yaml
# Should fail with: "Cannot export official Docker catalog 'docker-mcp'"
```