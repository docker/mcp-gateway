# Docker MCP Gateway Telemetry

## Overview

The Docker MCP Gateway now includes comprehensive OpenTelemetry (OTEL) instrumentation to provide visibility into Model Context Protocol (MCP) operations. This telemetry system tracks all interactions between AI clients and MCP servers, providing metrics and distributed tracing for debugging, monitoring, and performance analysis.

## Architecture

### Data Flow

The telemetry system captures data at multiple points in the MCP Gateway architecture:

```
AI Client (e.g., Claude Code)
    ↓
[Gateway Entry Point] → Records gateway startup
    ↓
[Interceptor Middleware] → Tracks list operations
    ↓
[Protocol Handlers] → Records tool/prompt/resource operations
    ↓
[MCP Servers] → Executes requested operations
    ↓
[Response Path] → Records duration, errors, results
    ↓
[OTEL Collector] → Aggregates and exports metrics
```

### Key Components

1. **Telemetry Package** (`internal/telemetry`)
   - Initializes OTEL tracer and meter from global providers
   - Defines all metrics and span creation functions
   - Provides recording functions for each operation type

2. **Interceptor Middleware** (`internal/interceptors`)
   - Tracks list operations (tools/list, prompts/list, resources/list, resourceTemplates/list)
   - Creates spans for tracking operation flow
   - Records metrics before operations reach handlers

3. **Protocol Handlers** (`internal/gateway/handlers.go`)
   - Instruments tool calls, prompt retrievals, and resource reads
   - Preserves server lineage in all telemetry
   - Records operation duration and errors

4. **Periodic Export** (`internal/gateway/run.go`)
   - Exports metrics every 30 seconds for long-running gateways
   - Configurable via `DOCKER_MCP_METRICS_INTERVAL` environment variable
   - Ensures metrics are available without waiting for shutdown

## Telemetry Coverage

### Gateway Operations

#### Startup and Lifecycle
- **`mcp.gateway.starts`** - Records when the gateway starts, including transport mode (stdio/sse/streaming)
- **`mcp.initialize`** - Records when the host initializes a connection with the gateway

#### Discovery Operations
When the gateway connects to MCP servers, it discovers their capabilities:
- **`mcp.tools.discovered`** - Number of tools available per server
- **`mcp.prompts.discovered`** - Number of prompts available per server
- **`mcp.resources.discovered`** - Number of resources available per server
- **`mcp.resource_templates.discovered`** - Number of resource templates available per server

### Client Operations

#### List Operations
When clients query available capabilities:
- **`mcp.list.tools`** - Client requests list of available tools
- **`mcp.list.prompts`** - Client requests list of available prompts
- **`mcp.list.resources`** - Client requests list of available resources
- **`mcp.list.resource_templates`** - Client requests list of available resource templates

### Protocol Operations

#### Tool Calls
- **`mcp.tool.calls`** - Counter of tool invocations
- **`mcp.tool.duration`** - Histogram of tool execution time (milliseconds)
- **`mcp.tool.errors`** - Counter of tool execution failures

#### Prompt Operations
- **`mcp.prompt.gets`** - Counter of prompt retrievals
- **`mcp.prompt.duration`** - Histogram of prompt operation time
- **`mcp.prompt.errors`** - Counter of prompt operation failures

#### Resource Operations
- **`mcp.resource.reads`** - Counter of resource reads
- **`mcp.resource.duration`** - Histogram of resource operation time
- **`mcp.resource.errors`** - Counter of resource operation failures

#### Resource Template Operations
- **`mcp.resource_template.reads`** - Counter of resource template reads
- **`mcp.resource_template.duration`** - Histogram of template operation time
- **`mcp.resource_template.errors`** - Counter of template operation failures

### CLI Direct Operations

When using the Docker CLI directly (not through the gateway):
- **`mcp.cli.tool.calls`** - Tool calls made directly from CLI
- **`mcp.cli.tool.duration`** - CLI tool execution duration
- **`mcp.cli.tools.discovered`** - Tools discovered via CLI commands

### Catalog Management

Operations for managing MCP server configurations:
- **`mcp.catalog.operations`** - Catalog management operations (ls, add, rm, create)
- **`mcp.catalog.operation.duration`** - Duration of catalog operations
- **`mcp.catalog.servers`** - Gauge showing number of servers in catalogs

## Metric Attributes

All metrics include contextual attributes for filtering and aggregation:

### Common Attributes
- **`mcp.server.name`** - Name of the MCP server handling the operation
- **`mcp.server.type`** - Type of server (docker, stdio, sse, unknown)

### Initialize Attributes
- **`mcp.client.name`** - Name of the connecting client (e.g. `claude-ai`)
- **`mcp.client.version`** - Version of the connecting client (e.g. `0.1.0`)

### Operation-Specific Attributes
- **`mcp.tool.name`** - Name of the tool being called
- **`mcp.prompt.name`** - Name of the prompt being retrieved
- **`mcp.resource.uri`** - URI of the resource being read
- **`mcp.operation.error`** - Error message if operation failed
- **`mcp.transport.mode`** - Gateway transport mode (stdio, sse, streaming)

## Distributed Tracing

The system creates spans for tracking operation flow:

### Span Hierarchy
```
gateway.run
├── tools/list
├── prompts/list
├── resources/list
├── resourceTemplates/list
├── mcp.tool.call
│   └── [tool execution]
├── mcp.prompt.get
│   └── [prompt retrieval]
├── mcp.resource.read
│   └── [resource read]
└── mcp.resource_template.read
    └── [template read]
```

### Span Attributes
Each span includes:
- Operation name and type
- Server information
- Duration
- Error details (if failed)
- Input parameters (tool name, prompt name, resource URI)

## Server Lineage Preservation

A key feature of the telemetry system is preserving the origin server for all operations. This allows you to:
- Track which MCP servers are most utilized
- Identify performance bottlenecks per server
- Monitor error rates by server
- Understand capability usage patterns

The server information is determined by analyzing the server configuration:
- **Docker containers**: Identified by image name
- **SSE endpoints**: Identified by URL
- **Stdio commands**: Identified by command path
- **Unknown**: Fallback for unrecognized configurations

## Long-Running Gateway Support

The MCP Gateway operates differently from typical Docker CLI commands. While most Docker commands are short-lived (running for seconds or minutes), the `docker mcp gateway run` command creates a long-running process that maintains persistent connections with AI clients. These gateway processes can run for hours, days, or even weeks as long as the client (such as Claude Code) remains connected.

Docker CLI's built-in OTEL exporter is designed for short-lived commands - it collects metrics throughout the command's execution and exports them when the command completes. This approach doesn't work well for the MCP Gateway because metrics would only be visible when the gateway shuts down, potentially days or weeks after interesting events occurred.

To address this difference, the MCP Gateway implements periodic metric export:

1. **Periodic Metric Export**: Metrics are exported every 30 seconds (configurable via `DOCKER_MCP_METRICS_INTERVAL`)
2. **Applies to All Transport Modes**: Whether using stdio, SSE, or streaming transports, all gateway processes are long-lived
3. **Non-blocking Operations**: Telemetry never blocks MCP operations
4. **Memory Efficiency**: Metrics are aggregated efficiently to minimize overhead
5. **Graceful Shutdown**: Final metric export still occurs on gateway termination

## Configuration

### Checking Current Configuration

View your Docker context's OTEL settings:
```bash
docker context inspect | jq '.[].Metadata.otel'
```

Typical output for Docker Desktop:
```json
{
  "OTEL_EXPORTER_OTLP_ENDPOINT": "unix:///Users/username/.docker/run/user-analytics.otlp.grpc.sock"
}
```

### Environment Variables

- **`DOCKER_MCP_TELEMETRY_DEBUG`** - Enable debug logging for telemetry operations
- **`DOCKER_MCP_METRICS_INTERVAL`** - Set metric export interval (default: 30s)
- **`DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT`** - Override OTEL collector endpoint for testing
- **`OTEL_EXPORTER_OTLP_ENDPOINT`** - OTEL collector endpoint (inherited from Docker CLI)
- **`OTEL_EXPORTER_OTLP_HEADERS`** - Authentication headers for OTEL collector

### Debug Mode

Enable debug logging to see telemetry events:
```bash
export DOCKER_MCP_TELEMETRY_DEBUG=1
docker mcp gateway run --catalog catalog.yaml
```

Debug output appears on stderr and includes:
- Provider initialization status
- Tool call instrumentation events
- Metric recording confirmations

### Integration with Docker Desktop

The telemetry system integrates with Docker Desktop's existing OTEL infrastructure:
1. Uses Docker CLI's global OTEL providers
2. Inherits Docker Desktop's telemetry configuration
3. Exports to the same OTEL collector as other Docker operations
4. Appears alongside other Docker metrics in monitoring systems

### Docker Desktop vs Open Source Docker

The telemetry behavior differs between Docker Desktop and open source Docker installations:

**Docker Desktop**:
- The Docker context includes OTEL configuration with an endpoint (typically a Unix socket at `~/.docker/run/user-analytics.otlp.grpc.sock`)
- The Docker Desktop backend process receives and processes metrics
- Metrics are filtered by instrumentation scope and exported to analytics services
- Users can disable telemetry through Docker Desktop settings

**Open Source Docker** (without Docker Desktop):
- The Docker context lacks OTEL configuration, causing `dockerExporterOTLPEndpoint()` to return an empty endpoint
- Without an endpoint, no metric exporter is created (`dockerMetricExporter()` returns nil)
- The MCP Gateway telemetry code runs but metrics are not exported anywhere
- No external connections are made for telemetry purposes

This design ensures that telemetry only operates within the Docker Desktop environment where users have explicitly agreed to usage analytics. Open source Docker users experience no telemetry collection, maintaining complete privacy by default.

## Use Cases

### Performance Monitoring
- Track response times for different MCP servers
- Identify slow operations by percentile analysis
- Monitor operation volume and patterns

### Debugging
- Trace failed operations through distributed spans
- Identify error patterns by server or operation type
- Correlate gateway operations with server behavior

### Capacity Planning
- Understand which capabilities are most used
- Track growth in operation volume
- Identify servers that need scaling

### Security and Compliance
- Audit tool usage patterns
- Track access to sensitive resources
- Monitor for unusual operation patterns

## Testing Telemetry

### Quick Test

Use the provided test script for complete telemetry validation:
```bash
cd docs/telemetry/testing
./test-telemetry.sh --full
```

### Manual Testing

1. **Start an OTEL Collector**:

For debug output only:
```bash
docker run --rm -d --name otel-debug \
  -p 4317:4317 -p 4318:4318 \
  -v $(pwd)/docs/telemetry/testing/otel-collector-config.yaml:/config.yaml \
  otel/opentelemetry-collector:latest --config=/config.yaml
```

For Prometheus export (port 8889):
```bash
docker run --rm -d --name otel-prometheus \
  -p 4317:4317 -p 4318:4318 -p 8889:8889 \
  -v $(pwd)/docs/telemetry/testing/otel-collector-prometheus.yaml:/config.yaml \
  otel/opentelemetry-collector:latest --config=/config.yaml
```

2. **Run MCP Gateway with Custom Endpoint**:
```bash
export DOCKER_MCP_TELEMETRY_DEBUG=1
export DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
docker mcp gateway run --catalog ~/.docker/mcp/catalogs/docker-mcp.yaml \
  --transport sse --port 3000
```

3. **Make Tool Calls**:
Connect an MCP client to `http://localhost:3000/sse` and execute tools.

4. **View Telemetry**:
```bash
docker logs otel-debug | grep "mcp\."
```

### Expected Output

When telemetry is working correctly, you'll see:

**Traces**:
```
Span #0
    Trace ID       : 3482c4807337c24926eca041191e9d74
    Name           : mcp.tool.call
    Kind           : Client
    Status code    : Ok
Attributes:
     -> mcp.tool.name: Str(getPersonalNamespace)
     -> mcp.server.name: Str(dockerhub)
     -> mcp.server.type: Str(docker)
```

**Metrics**:
```
Metric #0
Descriptor:
     -> Name: mcp.tool.calls
     -> Description: Number of tool calls executed
NumberDataPoints #0
     -> mcp.server.name: Str(dockerhub)
     -> mcp.tool.name: Str(getPersonalNamespace)
Value: 2
```

## Troubleshooting

### No Telemetry Visible

1. **Check Provider Types**:
   ```bash
   DOCKER_MCP_TELEMETRY_DEBUG=1 docker mcp gateway run --dry-run
   ```
   Look for: `Tracer provider type: *trace.TracerProvider` (not noop)

2. **Verify Endpoint Configuration**:
   - Check Docker context: `docker context inspect`
   - Check environment: `echo $DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT`

3. **Ensure Tool Calls Are Made**:
   - Telemetry is event-driven
   - No tool calls = no telemetry
   - Use SSE mode with an MCP client to trigger calls

### Debug Logging Not Appearing

- Ensure `DOCKER_MCP_TELEMETRY_DEBUG=1` is exported
- Check stderr output (debug logs go to stderr, not stdout)
- Verify the plugin was rebuilt after adding telemetry

### Collector Connection Issues

- Verify collector is running: `docker ps | grep otel`
- Check port availability: `lsof -i :4317`
- Test connectivity: `telnet localhost 4317`

## Privacy and Security

The telemetry system is designed with privacy in mind:
- **No PII**: No personally identifiable information is collected
- **No Content**: Operation contents (arguments, results) are not recorded
- **Metadata Only**: Only operation metadata and performance metrics
- **Local Control**: All telemetry can be disabled via Docker Desktop settings

## Performance Impact

The telemetry implementation is optimized for minimal overhead:
- Target: <1% performance impact
- Non-blocking metric recording
- Efficient attribute handling
- Batched metric exports
- No synchronous network calls in operation path

## Development Guidelines

### Adding New Telemetry

When adding telemetry to new operations:

1. **Use Existing Providers**:
   ```go
   tracer := otel.GetTracerProvider().Tracer("github.com/docker/mcp-gateway")
   meter := otel.GetMeterProvider().Meter("github.com/docker/mcp-gateway")
   ```

2. **Preserve Server Lineage**:
   Always include server attribution in spans and metrics:
   ```go
   attrs := []attribute.KeyValue{
       attribute.String("mcp.server.name", serverConfig.Name),
       attribute.String("mcp.server.type", inferServerType(serverConfig)),
       attribute.String("mcp.tool.name", toolName),
   }
   ```

3. **Non-Blocking Operations**:
   Telemetry must never block or fail operations:
   ```go
   if span != nil {
       defer span.End()
   }
   // Continue even if telemetry fails
   ```

4. **Debug Support**:
   Add debug logging behind the environment flag:
   ```go
   if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
       fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Recording tool call: %s\n", toolName)
   }
   ```

### Testing Changes

1. **Build the Plugin**:
   ```bash
   make docker-mcp
   ```

2. **Run the Test Suite**:
   ```bash
   ./docs/telemetry/testing/test-telemetry.sh --full
   ```

3. **Verify with Real Tool Calls**:
   Test in SSE mode with an actual MCP client

4. **Check Both Traces and Metrics**:
   Ensure both appear correctly in the collector output

## Future Enhancements

While the current implementation provides comprehensive coverage, potential future enhancements include:

### MCP Protocol Operations (Planned)
- **Sampling Operations**: Telemetry for when MCP servers request LLM completions from the MCP host via the client
- **Prompting Operations**: Metrics for when MCP servers request user input from the MCP host via the client
- **Roots Operations**: Tracking of filesystem boundaries where servers are allowed to operate

### Infrastructure Metrics
- Session-level metrics and tracking
- Connection pool metrics
- Interceptor execution metrics
- Advanced sampling strategies
- Custom dashboard templates

## Summary

The Docker MCP Gateway telemetry system provides complete visibility into MCP operations, from client requests through server execution. By leveraging OpenTelemetry standards and integrating with Docker Desktop's existing infrastructure, it enables effective monitoring, debugging, and optimization of AI-powered workflows using MCP servers.

The system balances comprehensive coverage with performance efficiency, ensuring that telemetry enhances rather than hinders the gateway's operation. With built-in support for long-running processes and careful preservation of server lineage, it provides the insights needed to operate MCP infrastructure effectively at scale.

## References

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/) - Core OTEL concepts and SDKs
- [Docker CLI Telemetry](https://github.com/docker/cli/tree/master/cli/command) - Docker's telemetry implementation
- [MCP Protocol Specification](https://modelcontextprotocol.io/) - Model Context Protocol details
- [MCP Gateway Repository](https://github.com/docker/mcp-gateway) - Source code and issues