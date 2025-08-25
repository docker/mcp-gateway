# Testing Improvement Plan

## Overview

This document outlines a comprehensive plan to improve unit test coverage for the Docker MCP Gateway project. The focus is on addressing gaps in unit test coverage while maintaining the excellent testing patterns already established in the codebase.

## Current State Assessment

### ✅ **Well-Tested Areas**
- Configuration parsing and validation (`adapters_test.go`)
- Secret management logic (`kubernetes_secret_manager_test.go`, `secret_naming_test.go`)
- Type validation and interface compliance (`types_test.go`)
- Templating and environment resolution (`standalone_test.go`)

### 🔴 **Critical Gaps**
- Core provisioning logic unit tests
- Container runtime mock implementations
- Error handling scenarios
- Resource management validation

## Improvement Plan

### **Phase 1: Critical Core Logic (Weeks 1-2)**

#### Priority 1.1: Complete Kubernetes Provisioner Unit Tests
**File**: `cmd/docker-mcp/internal/gateway/provisioners/kubernetes_provisioner_test.go`

```go
// Missing unit tests to add:
func TestKubernetesProvisioner_PreValidateDeployment_ValidationCases(t *testing.T)
func TestKubernetesProvisioner_PreValidateDeployment_ErrorScenarios(t *testing.T)
func TestKubernetesProvisioner_buildContainerSpec_AllFieldVariations(t *testing.T)
func TestKubernetesProvisioner_buildContainerSpec_SecretIntegration(t *testing.T)
func TestKubernetesProvisioner_buildContainerSpec_ResourceLimits(t *testing.T)
func TestKubernetesProvisioner_buildContainerSpec_Labels(t *testing.T)
func TestKubernetesProvisioner_extractResourceLimits(t *testing.T)
func TestKubernetesProvisioner_ErrorHandling(t *testing.T)
```

**Test Coverage Goals:**
- `PreValidateDeployment()`: Test all validation rules, edge cases, error conditions
- `buildContainerSpec()`: Test all field mappings, defaults, transformations
- Resource limit extraction and validation
- Label/annotation generation logic
- Configuration resolution error handling

#### Priority 1.2: Fix Container Runtime Mock Implementations
**File**: `cmd/docker-mcp/internal/gateway/container_runtime_test.go`

**Current Issue:**
```go
// These methods return "not implemented" errors
func (m *mockContainerRuntime) StartContainer(...) error {
    return fmt.Errorf("StartContainer not implemented in mock")
}
func (m *mockContainerRuntime) StopContainer(...) error {
    return fmt.Errorf("StopContainer not implemented in mock")
}
```

**Required Changes:**
- Implement proper mock behavior for `StartContainer`, `StopContainer`
- Add configurable mock responses for different test scenarios
- Test container spec validation in runtime layer
- Test error propagation and handling

```go
// New tests to add:
func TestMockContainerRuntime_StartContainer_Success(t *testing.T)
func TestMockContainerRuntime_StartContainer_Failure(t *testing.T)
func TestMockContainerRuntime_StopContainer_Variations(t *testing.T)
func TestContainerRuntime_SpecValidation(t *testing.T)
```

### **Phase 2: Runtime Layer Coverage (Week 3)**

#### Priority 2.1: Kubernetes Runtime Unit Tests
**File**: `cmd/docker-mcp/internal/gateway/runtime/kubernetes_test.go`

**Missing Unit Tests:**
```go
func TestKubernetesRuntime_buildPodSpec_BasicFields(t *testing.T)
func TestKubernetesRuntime_buildPodSpec_SecretMounts(t *testing.T)
func TestKubernetesRuntime_buildPodSpec_VolumeMounts(t *testing.T)
func TestKubernetesRuntime_buildPodSpec_ResourceConstraints(t *testing.T)
func TestKubernetesRuntime_generateLabels_StandardLabels(t *testing.T)
func TestKubernetesRuntime_generateLabels_CustomLabels(t *testing.T)
func TestKubernetesRuntime_sanitizeName_EdgeCases(t *testing.T)
func TestKubernetesRuntime_validateContainerSpec_ValidationRules(t *testing.T)
func TestKubernetesRuntime_inspectImage_ParseVariations(t *testing.T)
```

**Focus Areas:**
- Pod spec building logic (all field mappings)
- Label and selector generation
- Name sanitization and validation
- Container spec validation rules
- Image inspection logic

### **Phase 3: Error Handling & Edge Cases (Week 4)**

#### Priority 3.1: Comprehensive Error Scenario Tests

**For Each Major Component:**
- Invalid configuration handling
- Resource constraint violations  
- Network/timeout scenarios (unit test level)
- Malformed input validation
- Cleanup on failure scenarios

```go
// Examples across different test files:
func TestKubernetesProvisioner_PreValidateDeployment_InvalidSpecs(t *testing.T)
func TestAdaptServerConfigToSpec_MalformedConfig(t *testing.T)
func TestKubernetesRuntime_ResourceValidation_Failures(t *testing.T)
func TestSecretManager_MissingSecrets_Handling(t *testing.T)
```

#### Priority 3.2: Resource Management Unit Tests

**Missing Coverage:**
- CPU/Memory limit parsing and validation
- Resource constraint enforcement (unit level)
- Resource specification building
- Default resource assignment logic

### **Phase 4: Proxy & Handler Coverage (Week 5)**

#### Priority 4.1: Proxy Functionality Unit Tests
**File**: `cmd/docker-mcp/internal/gateway/proxies/proxy_spec_test.go`

**Current State**: Only tests spec parsing (1 test function)

**Missing Unit Tests:**
```go
func TestProxy_ValidationLogic(t *testing.T)
func TestProxy_ProtocolHandling_HTTP(t *testing.T)
func TestProxy_ProtocolHandling_TCP(t *testing.T)
func TestProxy_ErrorScenarios(t *testing.T)
func TestProxy_ConfigurationMerging(t *testing.T)
```

#### Priority 4.2: Complete Telemetry Unit Tests
**File**: `cmd/docker-mcp/internal/gateway/handlers_telemetry_test.go`

**Current Issue**: Contains skipped placeholder test
```go
func TestHandlerInstrumentationIntegration(t *testing.T) {
    t.Skip("Full integration test will be enabled after handler instrumentation is complete")
}
```

**Action**: Convert to proper unit test or remove if truly integration-focused

### **Phase 5: Quality Improvements (Week 6)**

#### Priority 5.1: Test Organization & Cleanup
- Review and refactor large test functions
- Ensure consistent table-driven test patterns
- Add missing helper functions
- Improve test documentation

#### Priority 5.2: Test Coverage Validation
- Run coverage analysis on each improved file
- Ensure >85% unit test coverage for core logic
- Document remaining uncovered edge cases

## Implementation Guidelines

### **Testing Patterns to Follow**

1. **Table-Driven Tests** (maintain existing pattern):
```go
tests := []struct {
    name        string
    input       InputType
    expected    ExpectedType
    expectError bool
}{
    // test cases...
}
```

2. **Mock Configuration**:
```go
type testMock struct {
    returnValue  interface{}
    returnError  error
    capturedArgs []interface{}
}
```

3. **Error Testing**:
```go
func TestFunction_ErrorCases(t *testing.T) {
    // Test each error condition separately
    // Use require.Error() for expected errors
    // Use assert.Contains() for error message validation
}
```

### **Test Organization Standards**

- One test file per source file
- Group related tests with shared setup
- Use `t.Helper()` in test utility functions
- Consistent naming: `TestStructName_MethodName_Scenario`

### **Coverage Goals by Component**

| Component | Current Coverage | Target Coverage | Priority |
|-----------|------------------|-----------------|----------|
| Kubernetes Provisioner Core | ~40% | >85% | Critical |
| Container Runtime Logic | ~30% | >80% | Critical |
| Kubernetes Runtime | ~50% | >80% | High |
| Proxy Functionality | ~20% | >75% | Medium |
| Error Handling | ~25% | >80% | High |

## Success Metrics

### **Phase Completion Criteria**

1. **Phase 1**: All critical provisioning logic has comprehensive unit tests
2. **Phase 2**: Runtime layer achieves >80% unit test coverage
3. **Phase 3**: Error scenarios are thoroughly tested across all components
4. **Phase 4**: Supporting components (proxy, telemetry) have complete unit coverage
5. **Phase 5**: Overall codebase maintains >85% unit test coverage

### **Quality Gates**

- All new tests must follow established patterns
- No skipped tests without documented justification
- All mock implementations must be complete and configurable
- Test execution time should remain under 30 seconds for unit tests

## Timeline Summary

| Week | Phase | Focus Area | Deliverables |
|------|-------|------------|-------------|
| 1-2 | Phase 1 | Critical Core Logic | Complete provisioner + runtime mock tests |
| 3 | Phase 2 | Runtime Layer | Kubernetes runtime unit test coverage |
| 4 | Phase 3 | Error Handling | Comprehensive error scenario tests |
| 5 | Phase 4 | Supporting Components | Proxy, telemetry unit tests |
| 6 | Phase 5 | Quality & Validation | Coverage analysis, documentation |

## Maintenance

This plan should be reviewed and updated quarterly to ensure continued test quality and coverage as the codebase evolves.