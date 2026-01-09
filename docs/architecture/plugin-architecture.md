# Plugin Architecture

This document describes the plugin architecture for MCP Gateway, which enables extensibility through a unified interface that works across different deployment models.

## Overview

The plugin architecture provides a way to extend MCP Gateway functionality without modifying the core codebase. Plugins can provide capabilities like:

- **Telemetry** - Custom metrics, tracing, and logging backends
- **Authentication** - Custom authentication providers
- **Authorization** - Access control and policy enforcement
- **Audit** - Event logging and compliance
- **Credential Storage** - Secure credential management

## Design Goals

1. **Deployment Agnostic** - Same plugin interface works for Desktop (subprocess) and Kubernetes (sidecar)
2. **Language Independent** - Plugins communicate via HTTP/JSON, enabling any language
3. **Loosely Coupled** - Gateway and plugins can evolve independently
4. **Testable** - Easy to mock plugins for testing
5. **Observable** - Lifecycle hooks for monitoring plugin health

## Architecture Diagram

```
┌───────────────────────────────────────────────────────────────────────┐
│                             MCP Gateway                               │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                       Plugin Registry                           │  │
│  │                                                                 │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │  │
│  │  │  Telemetry  │  │    Auth     │  │   Audit     │   ...        │  │
│  │  │   Plugin    │  │  Provider   │  │    Sink     │              │  │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │  │
│  └─────────┼────────────────┼────────────────┼─────────────────────┘  │
│            │                │                │                        │
│  ┌─────────▼────────────────▼────────────────▼─────────────────────┐  │
│  │                    PluginClient Interface                       │  │
│  │                                                                 │  │
│  │  - Call(method, params) → result                                │  │
│  │  - Close()                                                      │  │
│  └─────────┬────────────────────────────────┬──────────────────────┘  │
│            │                                │                         │
└────────────┼────────────────────────────────┼─────────────────────────┘
             │                                │
    ┌────────▼────────┐              ┌────────▼────────┐
    │   Subprocess    │              │    Sidecar      │
    │    Transport    │              │   Transport     │
    │                 │              │                 │
    │  - Exec binary  │              │  - HTTP client  │
    │  - stdio/HTTP   │              │  - localhost    │
    │  - Lifecycle    │              │  - K8s managed  │
    └────────┬────────┘              └────────┬────────┘
             │                                │
    ┌────────▼────────┐              ┌────────▼────────┐
    │  Desktop Mode   │              │ Kubernetes Mode │
    │                 │              │                 │
    │  Plugin binary  │              │Sidecar container│
    │  as subprocess  │              │  in same pod    │
    └─────────────────┘              └─────────────────┘
```

## Core Components

### PluginClient Interface

The `PluginClient` interface abstracts communication with plugins:

```go
type PluginClient interface {
    // Call invokes a method on the plugin
    Call(ctx context.Context, method string, params any) ([]byte, error)

    // Close shuts down the connection
    Close() error
}
```

### PluginTransport Interface

The `PluginTransport` interface defines how to establish connections:

```go
type PluginTransport interface {
    // Connect establishes a connection and returns a client
    Connect(ctx context.Context) (PluginClient, error)
}
```

### Plugin Registry

The registry manages plugin instances and provides type-safe access:

```go
type Registry struct {
    telemetryPlugin TelemetryPlugin
    // ... other plugin types
}

func (r *Registry) TelemetryPlugin() TelemetryPlugin
func (r *Registry) RegisterTelemetryPlugin(p TelemetryPlugin) error
```

### Plugin Loader

The loader handles plugin discovery and lifecycle:

```go
type Loader struct {
    plugins map[string]*loadedPlugin
    hooks   *PluginLifecycleHooks
}

func (l *Loader) Load(ctx context.Context, config PluginConfig) error
func (l *Loader) Unload(name string) error
func (l *Loader) Reload(ctx context.Context, name string) error
```

## Transport Implementations

### Subprocess Transport (Desktop)

For Desktop deployments, plugins run as subprocess:

1. Gateway spawns plugin binary
2. Plugin outputs `PORT=<port>` on stdout
3. Gateway connects via HTTP to localhost:port
4. Gateway manages plugin lifecycle (restart on crash)

**Configuration:**
```yaml
plugins:
  telemetry:
    type: subprocess
    subprocess:
      exec: /usr/local/bin/telemetry-plugin
      args: ["--port", "0"]
      env:
        LOG_LEVEL: debug
```

### Sidecar Transport (Kubernetes)

For Kubernetes deployments, plugins run as sidecars:

1. Kubernetes starts sidecar containers
2. Gateway connects via HTTP to localhost URLs
3. Kubernetes manages sidecar lifecycle
4. Gateway handles reconnection on failures

**Configuration:**
```yaml
plugins:
  telemetry:
    type: sidecar
    sidecar:
      url: http://localhost:8081
      headers:
        X-Plugin-Token: "${PLUGIN_TOKEN}"
```

## Plugin Communication Protocol

Plugins communicate via HTTP/JSON-RPC style calls:

### Request Format
```json
{
  "method": "record-counter",
  "params": {
    "name": "mcp.tool.calls",
    "value": 1,
    "attributes": {
      "mcp.tool.name": "docker_ps"
    }
  }
}
```

### Response Format
```json
{
  "result": "ok"
}
```

### Health Check
Plugins must implement a health endpoint:
- `GET /health` → `200 OK` when ready

### Call Endpoint
Plugins must implement a call endpoint:
- `POST /call` → Process method call

## Plugin Types

### TelemetryPlugin

Records metrics, traces, and logs:

```go
type TelemetryPlugin interface {
    RecordCounter(ctx, name, value, attrs)
    RecordHistogram(ctx, name, value, attrs)
    RecordGauge(ctx, name, value, attrs)
    Close() error
}
```

### AuthProvider (Planned)

Validates credentials and returns user principals:

```go
type AuthProvider interface {
    ValidateCredential(ctx, credential) (*UserPrincipal, error)
    Close() error
}
```

### AuditSink (Planned)

Records audit events:

```go
type AuditSink interface {
    LogEvent(ctx, event) error
    Close() error
}
```

## Lifecycle Management

### Plugin Startup

1. Load configuration
2. Create appropriate transport
3. Connect to plugin
4. Wait for health check
5. Register with registry
6. Call `OnStart` hook

### Plugin Shutdown

1. Call `Close()` on client
2. For subprocess: send SIGTERM, wait, then SIGKILL
3. For sidecar: just close HTTP client
4. Unregister from registry
5. Call `OnStop` hook

### Error Handling

1. On connection failure: retry with backoff
2. On call failure: return error to caller
3. On crash (subprocess): restart and call `OnRestart` hook

## Configuration

### Desktop Mode (Config File)

```yaml
plugins:
  telemetry:
    type: subprocess
    subprocess:
      exec: /usr/local/bin/telemetry-plugin
      args: ["--port", "0"]

  auth:
    type: subprocess
    subprocess:
      exec: /usr/local/bin/auth-plugin
```

### Kubernetes Mode (ConfigMap)

```yaml
plugins:
  telemetry:
    type: sidecar
    sidecar:
      url: http://localhost:8081

  auth:
    type: sidecar
    sidecar:
      url: http://localhost:8082
```

## Implementing a Plugin

### Go Example

```go
package main

import (
    "encoding/json"
    "net/http"
    "fmt"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    http.HandleFunc("/call", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Method string         `json:"method"`
            Params map[string]any `json:"params"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        // Handle method
        switch req.Method {
        case "record-counter":
            // Process counter
        }

        json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
    })

    // Listen on random port and output it
    listener, _ := net.Listen("tcp", "127.0.0.1:0")
    port := listener.Addr().(*net.TCPAddr).Port
    fmt.Printf("PORT=%d\n", port)

    http.Serve(listener, nil)
}
```

### Python Example

```python
from flask import Flask, request, jsonify
import sys

app = Flask(__name__)

@app.route('/health')
def health():
    return '', 200

@app.route('/call', methods=['POST'])
def call():
    data = request.json
    method = data.get('method')
    params = data.get('params', {})

    if method == 'record-counter':
        # Process counter
        pass

    return jsonify({'result': 'ok'})

if __name__ == '__main__':
    import socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(('127.0.0.1', 0))
    port = sock.getsockname()[1]
    sock.close()

    print(f'PORT={port}', flush=True)
    app.run(host='127.0.0.1', port=port)
```

## Security Considerations

1. **Subprocess Isolation** - Plugins run in separate processes with limited privileges
2. **Network Isolation** - Sidecar plugins only accessible via localhost
3. **Input Validation** - All plugin inputs are validated
4. **Credential Handling** - Sensitive data passed via environment variables
5. **Health Monitoring** - Unhealthy plugins are detected and can be restarted

## Testing

### Unit Testing

Mock the `PluginClient` interface:

```go
type MockPluginClient struct {
    CallFunc func(ctx context.Context, method string, params any) ([]byte, error)
}

func (m *MockPluginClient) Call(ctx context.Context, method string, params any) ([]byte, error) {
    return m.CallFunc(ctx, method, params)
}
```

### Integration Testing

Use the actual plugin with subprocess transport in tests:

```go
func TestTelemetryPlugin(t *testing.T) {
    loader := plugins.NewLoader(nil)
    err := loader.Load(ctx, plugins.PluginConfig{
        Name: "telemetry",
        Type: "subprocess",
        Subprocess: &plugins.SubprocessConfig{
            Exec: "./test-telemetry-plugin",
        },
    })
    require.NoError(t, err)
    defer loader.UnloadAll()

    client, _ := loader.Get("telemetry")
    result, err := client.Call(ctx, "record-counter", map[string]any{
        "name": "test.counter",
        "value": 1,
    })
    require.NoError(t, err)
}
```

## Future Enhancements

1. **gRPC Support** - Add gRPC transport for performance-critical plugins
2. **Plugin Marketplace** - Curated collection of community plugins
3. **Hot Reload** - Reload plugins without gateway restart
4. **Plugin Versioning** - Version compatibility checking
5. **Plugin Dependencies** - Plugins that depend on other plugins
