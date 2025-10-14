# MCP Manager Gateway Architecture

## Design Principles
- **Independent**: No dependency on Docker Inc. infrastructure
- **Flexible**: Works with containers OR local processes
- **Lightweight**: Minimal overhead, efficient resource usage
- **Protocol-compliant**: Full MCP protocol support

## Architecture Overview

```
┌─────────────────┐
│   AI Client     │ (Claude Code, etc.)
│ (MCP Protocol)  │
└────────┬────────┘
         │ HTTP/SSE
         │
┌────────▼────────────────────────────────────────┐
│           MCP Manager Gateway                   │
│  ┌──────────────────────────────────────────┐  │
│  │  HTTP Server (Port 8080)                 │  │
│  │  - Handles MCP JSON-RPC messages         │  │
│  │  - Manages sessions                      │  │
│  │  - Routes to appropriate servers         │  │
│  └──────────────┬───────────────────────────┘  │
│                 │                                │
│  ┌──────────────▼───────────────────────────┐  │
│  │  Server Manager                          │  │
│  │  - Spawns containers/processes on-demand │  │
│  │  - Manages lifecycle                     │  │
│  │  - Pipes stdin/stdout                    │  │
│  └──────────────┬───────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  │
         ┌────────┴────────┐
         │                 │
┌────────▼────────┐ ┌─────▼──────────┐
│  MCP Server     │ │  MCP Server    │
│  (Container)    │ │  (Container)   │
│  stdio-based    │ │  stdio-based   │
└─────────────────┘ └────────────────┘
```

## Components

### 1. Gateway HTTP Server (`internal/gateway`)
- **Responsibility**: Accept MCP client connections
- **Transport**: HTTP with optional SSE for streaming
- **Port**: Configurable (default 8080)
- **Protocol**: MCP JSON-RPC over HTTP POST

**Key Operations**:
- `POST /mcp` - Send MCP message to server
- `GET /mcp/stream` - SSE stream for server messages (optional)
- `GET /health` - Health check
- `GET /servers` - List available servers

### 2. Session Manager
- **Responsibility**: Manage client sessions and server associations
- **Lifecycle**: Creates session → spawns server → pipes I/O → cleanup
- **State**: Track active sessions, server processes, cleanup

### 3. Server Spawner
- **Responsibility**: Spawn MCP server containers or processes on-demand
- **Mode A**: Container (default) - uses Docker API
- **Mode B**: Process (fallback) - direct subprocess
- **No restart policies**: Containers/processes are ephemeral

### 4. I/O Bridge
- **Responsibility**: Bridge HTTP ↔ stdio
- **Client → Server**: Read HTTP request body → write to server stdin
- **Server → Client**: Read from server stdout → return in HTTP response
- **Buffering**: Handle streaming responses properly

## Request Flow

### Initial Connection
1. Client sends MCP initialization message via `POST /mcp`
2. Gateway creates session ID
3. Gateway spawns configured MCP server (container or process)
4. Gateway pipes initialization to server stdin
5. Server responds on stdout
6. Gateway returns response to client

### Subsequent Messages
1. Client includes session ID in requests
2. Gateway routes to existing server process
3. Message piped to server stdin
4. Response read from stdout
5. Returned to client

### Cleanup
1. Client disconnects or timeout
2. Gateway terminates server process/container
3. Cleanup resources
4. Log session metrics

## Configuration

### Gateway Config (`gateway.yaml`)
```yaml
gateway:
  host: "0.0.0.0"
  port: 8080
  mode: "container"  # or "process"
  session_timeout: 30m
  
servers:
  - name: filesystem
    enabled: true
    image: mcp/filesystem:latest  # for container mode
    command: []
    args: ["/workspace"]
    volumes:
      - host_path: ${PWD}
        container_path: /workspace
```

## Implementation Phases

### Phase 1: Basic HTTP Gateway
- HTTP server with MCP endpoints
- Session management
- Basic request/response handling

### Phase 2: Container Spawning
- On-demand container creation
- Stdin/stdout piping
- Proper cleanup

### Phase 3: Protocol Compliance
- Full MCP JSON-RPC support
- Error handling
- SSE streaming (optional)

### Phase 4: Production Features
- Logging and metrics
- Health checks
- Graceful shutdown
- Resource limits

## Advantages Over Docker Gateway

1. **Independence**: No reliance on Docker Inc.
2. **Flexibility**: Can run servers as containers OR processes
3. **Simplicity**: Single binary, clear configuration
4. **Extensibility**: Easy to add features and customizations
5. **Transparency**: Open source, understandable codebase

## Key Differences from Current Implementation

**Before (Broken)**:
- Containers run continuously with restart policies
- No client connection management
- Direct container-to-client communication assumed

**After (Gateway Pattern)**:
- Gateway runs continuously (not MCP servers)
- Containers spawned on-demand per session
- HTTP ↔ stdio bridging layer
- Clean lifecycle management
