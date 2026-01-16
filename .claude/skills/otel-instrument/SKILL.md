---
name: OpenTelemetry Instrumentation Extension
description: Extend OpenTelemetry instrumentation when new functionality is added to the MCP Gateway. Use when (1) new operations/functions are added, (2) reviewing code for missing instrumentation, (3) user requests otel/telemetry additions, or (4) working with state-changing operations. Analyzes git diff, suggests instrumentation points following project standards in docs/telemetry/README.md, implements with approval, writes tests, updates documentation, and verifies with debug logging and docker logs.
---

# OpenTelemetry Instrumentation Extension

Automatically extend OpenTelemetry instrumentation for new functionality in the MCP Gateway, following established patterns documented in `docs/telemetry/README.md`.

## When to Use This Skill

Automatically apply when:
- New state-changing operations added (Create, Update, Delete, Push, Pull, Add, Remove, etc.)
- New CLI commands added to `cmd/docker-mcp/`
- New packages with operations in `pkg/`
- User mentions "otel", "telemetry", "instrumentation", "metrics", or "tracing"
- Code changes that modify state (database, files, containers, configuration)
- Reviewing code for telemetry coverage

## Workflow

### Phase 1: Analysis & Suggestion
1. **Read project telemetry standards**:
   - Read `docs/telemetry/README.md` "Development Guidelines" section
   - Read `pkg/telemetry/telemetry.go` to understand existing patterns and metrics

2. **Identify scope** using git diff:
   - Find new/changed files in `pkg/` and `cmd/docker-mcp/`
   - Identify functions performing state-changing operations
   - Infer domain from package structure (e.g., `pkg/foo/` → domain: `foo`)

3. **Categorize findings**:
   - Operations in existing domains (use existing metrics)
   - Operations in new domains (need new metrics)

4. **Present suggestions** to user:
   - List each function needing instrumentation with file:line reference
   - Specify operation type (create, update, delete, etc.)
   - Identify domain and whether metrics exist
   - Show what will be added (instrumentation code, metrics, docs, tests)

### Phase 2: Implementation (After User Approval)

Execute in this order:

1. **Make changes**: Instrument operations and add metrics to `pkg/telemetry/telemetry.go`
2. **Verify**: Build, run with OTEL collector, check `docker logs otel-debug` output
3. **Write tests**: Add tests to `pkg/telemetry/telemetry_test.go`
4. **Update docs**: Add metrics/operations to `docs/telemetry/README.md`
5. **Run tests**: Execute `make test`
6. **Final verification**: Run `./docs/telemetry/testing/test-telemetry.sh`

## Key Principles

Follow the project's documented guidelines from `docs/telemetry/README.md`:

1. **Use Existing Providers**: Get global tracer/meter from OTEL
2. **Preserve Server Lineage**: Include server attribution in all telemetry
3. **Non-Blocking Operations**: Telemetry never blocks or fails operations
4. **Debug Support**: Add logging behind `DOCKER_MCP_TELEMETRY_DEBUG`
5. **Follow Naming Conventions**: Use `mcp.<domain>.<field>` pattern
6. **Deferred Success Tracking**: Only set success on clean completion

## Where to Find Information

**ALWAYS read these files before suggesting changes:**

1. **`docs/telemetry/README.md`** - Source of truth for:
   - Development guidelines (lines 419-474)
   - Existing metrics and attributes
   - Testing procedures
   - Naming conventions

2. **`pkg/telemetry/telemetry.go`** - Understand:
   - Existing metric instruments
   - Recording function patterns
   - Helper functions for spans
   - Init() structure

3. **`pkg/telemetry/telemetry_test.go`** - See:
   - Testing patterns
   - How to verify metrics

## Instrumentation Pattern

Use the simple defer pattern from `cmd/docker-mcp/catalog/create.go`:

```go
func OperationName(ctx context.Context, identifier string, ...) error {
    telemetry.Init()
    start := time.Now()
    var success bool
    defer func() {
        duration := time.Since(start)
        telemetry.Record<Domain>Operation(ctx, "operation_name", identifier,
            float64(duration.Milliseconds()), success)
    }()

    // ... operation logic ...

    // Optional: Record resource counts
    telemetry.Record<Domain><Resources>(ctx, identifier, int64(count))

    success = true
    return nil
}
```

Note: If identifier is generated during execution, the defer captures its final value.

## Adding New Domains

When instrumentation is needed for a new domain (e.g., new package `pkg/newdomain/`):

### 1. Add Metric Instruments in `pkg/telemetry/telemetry.go`

In the `Init()` function, add global variables and create metric instruments:

```go
var (
    // ... existing metrics ...

    // New domain metrics
    newdomainOperations        metric.Int64Counter
    newdomainOperationDuration metric.Float64Histogram
    newdomainResources         metric.Int64Gauge  // If managing resources
)

func Init() {
    // ... existing init code ...

    newdomainOperations, _ = meter.Int64Counter(
        "mcp.newdomain.operations",
        metric.WithDescription("New domain operations count"),
    )
    newdomainOperationDuration, _ = meter.Float64Histogram(
        "mcp.newdomain.operation.duration",
        metric.WithDescription("New domain operation duration in milliseconds"),
    )
    newdomainResources, _ = meter.Int64Gauge(
        "mcp.newdomain.resources",
        metric.WithDescription("Number of resources in new domain"),
    )
}
```

### 2. Add Recording Functions in `pkg/telemetry/telemetry.go`

After the `Init()` function, add recording functions:

```go
func RecordNewdomainOperation(ctx context.Context, operation, identifier string, durationMs float64, success bool) {
    if newdomainOperations == nil || newdomainOperationDuration == nil {
        return
    }
    attrs := []attribute.KeyValue{
        attribute.String("mcp.newdomain.operation", operation),
        attribute.String("mcp.newdomain.id", identifier),  // or .name, .ref as appropriate
        attribute.Bool("mcp.newdomain.success", success),
    }
    newdomainOperations.Add(ctx, 1, metric.WithAttributes(attrs...))
    newdomainOperationDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// Optional: Add resource counting function if applicable
func RecordNewdomainResources(ctx context.Context, identifier string, count int64) {
    if newdomainResources == nil {
        return
    }
    attrs := []attribute.KeyValue{
        attribute.String("mcp.newdomain.id", identifier),
    }
    newdomainResources.Record(ctx, count, metric.WithAttributes(attrs...))
}
```

### 3. Write Tests in `pkg/telemetry/telemetry_test.go`

Add test cases following existing patterns:

```go
func TestRecordNewdomainOperation(t *testing.T) {
    spanRecorder, metricReader := setupTestTelemetry(t)
    Init()
    ctx := context.Background()

    // Test successful operation
    RecordNewdomainOperation(ctx, "create", "test-id", 123.45, true)

    // Collect and verify metrics
    var rm metricdata.ResourceMetrics
    err := metricReader.Collect(ctx, &rm)
    require.NoError(t, err)

    // Find and verify the counter metric
    foundCounter := false
    foundHistogram := false
    for _, sm := range rm.ScopeMetrics {
        for _, m := range sm.Metrics {
            if m.Name == "mcp.newdomain.operations" {
                foundCounter = true
                sum := m.Data.(metricdata.Sum[int64])
                require.Len(t, sum.DataPoints, 1)
                assert.Equal(t, int64(1), sum.DataPoints[0].Value)

                // Verify attributes
                attrs := sum.DataPoints[0].Attributes
                assert.Contains(t, attrs.ToSlice(), attribute.String("mcp.newdomain.operation", "create"))
                assert.Contains(t, attrs.ToSlice(), attribute.String("mcp.newdomain.id", "test-id"))
                assert.Contains(t, attrs.ToSlice(), attribute.Bool("mcp.newdomain.success", true))
            }
            if m.Name == "mcp.newdomain.operation.duration" {
                foundHistogram = true
                histogram := m.Data.(metricdata.Histogram[float64])
                require.Len(t, histogram.DataPoints, 1)
                assert.Equal(t, float64(123.45), histogram.DataPoints[0].Sum)
            }
        }
    }

    assert.True(t, foundCounter, "Counter metric not found")
    assert.True(t, foundHistogram, "Histogram metric not found")
}
```

### 4. Update Documentation in `docs/telemetry/README.md`

Add a new section for the domain in the appropriate location. Follow the existing format:

```markdown
### New Domain Operations

Operations for managing [description of what this domain does]:
- **`mcp.newdomain.operations`** - New domain operations (create, update, delete, etc.)
- **`mcp.newdomain.operation.duration`** - Duration of new domain operations
- **`mcp.newdomain.resources`** - Gauge showing number of resources in new domain

#### New Domain Attributes
- **`mcp.newdomain.operation`** - Type of operation (create, update, delete, etc.)
- **`mcp.newdomain.id`** - ID of the resource
- **`mcp.newdomain.success`** - Boolean indicating operation success
```

## Verification

After making changes, verify telemetry output:

```bash
# Build
make docker-mcp

# Start OTEL collector
docker run --rm -d --name otel-debug \
  -p 4317:4317 -p 4318:4318 \
  -v $(pwd)/docs/telemetry/testing/otel-collector-config.yaml:/config.yaml \
  otel/opentelemetry-collector:latest --config=/config.yaml

# Run with telemetry enabled
export DOCKER_MCP_TELEMETRY_DEBUG=1
export DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
docker mcp [command]

# Check collector output
docker logs otel-debug | grep "mcp.newdomain"

# Cleanup
docker stop otel-debug
```

Verify the output matches expectations before proceeding to write tests and docs.

## Analysis Strategy

When analyzing git diff:

1. **Look for operation verbs** in function names:
   - Create, Add, Update, Modify, Set, Configure, Register
   - Delete, Remove, Unregister, Clear
   - Push, Pull, Export, Import, Sync
   - Start, Stop, Run, Execute

2. **Look for state changes**:
   - Database operations (DAO/DB calls)
   - File system operations (create, delete, write)
   - Container operations (start, stop, create, delete)
   - Configuration changes (save, update)

3. **Infer domain** from file path:
   - `pkg/workingset/` → domain: `profile`
   - `pkg/catalog_next/` → domain: `catalog_next`
   - `cmd/docker-mcp/server/` → domain: `server`
   - Pattern: use logical grouping name

4. **Check if telemetry exists**:
   - Search `pkg/telemetry/telemetry.go` for `Record<Domain>` functions
   - If exists: use existing metrics
   - If not: propose new domain metrics

## Implementation Checklist

Follow this sequence:

- [ ] Read patterns from `docs/telemetry/README.md`
- [ ] Instrument operations with simple defer pattern
- [ ] Add new metrics to `pkg/telemetry/telemetry.go` (if new domain)
- [ ] Verify with `docker logs otel-debug` - confirm output matches expectations
- [ ] Write tests in `pkg/telemetry/telemetry_test.go`
- [ ] Update `docs/telemetry/README.md` with new metrics
- [ ] Run `make test` - verify tests pass
- [ ] Run `./docs/telemetry/testing/test-telemetry.sh` - final verification

## Important Notes

- Read documentation first
- Follow existing patterns
- Ask for approval before implementing
- Verify early and often with collector logs
