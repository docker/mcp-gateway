# Official Registry Integration

## Architecture Overview

This diagram shows the interactions between the core components of the MCP Gateway system:

```mermaid
graph TB
    Client[AI Client] --> Gateway[MCP Gateway]
    
    subgraph "Gateway Core"
        Gateway --> ClientPool[ClientPool]
        Gateway --> Config[Configurations]
        Gateway --> Interceptor[Interceptors]
        Gateway --> Remotes[Remotes]
    end
    
    subgraph "Local MCP Server"
        LocalServer[MCP Server<br/>Docker Container]
    end
    
    subgraph "Remote MCP Server"
        RemoteServer[Remote MCP Server<br/>HTTP/SSE]
    end
    
    ClientPool --> LocalServer
    Remotes --> RemoteServer
    
    Config --> ClientPool
    Config --> Interceptor
    Config --> Remotes
    
    Interceptor --> ClientPool
    Interceptor --> Remotes
    Interceptor -.->|logs/monitors| LocalServer
    Interceptor -.->|logs/monitors| RemoteServer
    
    Gateway -.->|manages lifecycle| LocalServer
    Gateway -.->|manages connections| RemoteServer
    
    classDef client fill:#e1f5fe
    classDef gateway fill:#f3e5f5
    classDef server fill:#e8f5e8
    classDef remote fill:#fff3e0
    
    class Client client
    class Gateway,ClientPool,Config,Interceptor,Remotes gateway
    class LocalServer server
    class RemoteServer remote
```

## Component Interactions

### Gateway
- Central orchestrator that receives requests from AI clients
- Manages the lifecycle of MCP servers running in Docker containers
- Routes messages between clients and appropriate servers

### ClientPool
- Manages connections to multiple MCP servers
- Handles connection pooling and load balancing
- Maintains persistent connections to containerized servers

### Configurations
- Provides configuration data to ClientPool and Interceptors
- Manages server definitions, transport modes, and security settings
- Loads from catalog files and runtime configuration

### Interceptors
- Intercept and monitor communication between clients and servers
- Handle logging, security validation, and call inspection
- Can modify or block requests based on policies

### Remotes
- Manages connections to remote MCP servers via HTTP/SSE
- Handles authentication and custom headers for remote endpoints
- Provides unified interface for both local and remote MCP servers
- Supports load balancing across multiple remote instances

### MCP Servers
- Run as isolated Docker containers
- Each server provides specific tools and capabilities
- Communicate with the gateway via stdio, SSE, or streaming protocols

### Remote MCP Servers
- External MCP servers accessible via HTTP/SSE protocols
- Can be hosted anywhere with network connectivity
- Support custom authentication and header configurations
- Integrated seamlessly alongside containerized servers
