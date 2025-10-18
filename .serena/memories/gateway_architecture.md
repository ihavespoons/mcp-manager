# MCP Manager Gateway Architecture

## Design Principles
- **Independent**: No dependency on Docker Inc. infrastructure
- **Flexible**: Works with containers AND local processes simultaneously
- **Lightweight**: Minimal overhead, efficient resource usage
- **Protocol-compliant**: Full MCP protocol support
- **Clean Lifecycle**: Proper container cleanup on shutdown
- **Per-Server Mode Detection**: Automatic selection of container vs process mode
- **Auto-Sync**: Keeps Claude Code config in sync with mcp-config.yaml

## Architecture Overview

```
┌─────────────────┐
│   AI Client     │ (Claude Code, etc.)
│ (MCP Protocol)  │
└────────┬────────┘
         │ HTTP (JSON-RPC)
         │
┌────────▼────────────────────────────────────────┐
│           MCP Manager Gateway                   │
│         (Mixed Mode: Auto-Detection)            │
│  ┌──────────────────────────────────────────┐  │
│  │  HTTP Server (Port 52080)                │  │
│  │  - Handles MCP JSON-RPC messages         │  │
│  │  - Path-based routing: /mcp/{server}     │  │
│  │  - Health check: /health                 │  │
│  │  - Server list: /servers                 │  │
│  └──────────────┬───────────────────────────┘  │
│                 │                                │
│  ┌──────────────▼───────────────────────────┐  │
│  │  Server Manager                          │  │
│  │  - Creates persistent server instances   │  │
│  │  - Per-server mode detection             │  │
│  │  - One instance per server name          │  │
│  │  - Manages lifecycle                     │  │
│  │  - Pipes stdin/stdout for JSON-RPC       │  │
│  │  - Handles cleanup on shutdown           │  │
│  └──────────────┬───────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  │
         ┌────────┴────────┐
         │                 │
┌────────▼────────┐ ┌─────▼──────────┐
│  MCP Server     │ │  MCP Server    │
│  (Container)    │ │  (Process)     │
│  if docker.image│ │  otherwise     │
│  stdio-based    │ │  stdio-based   │
│  Persistent     │ │  Persistent    │
└─────────────────┘ └────────────────┘
```

## Components

### 1. Gateway HTTP Server (`internal/gateway/gateway.go`)
- **Responsibility**: Accept MCP client connections
- **Transport**: HTTP with JSON-RPC 2.0
- **Port**: Configurable (default 52080)
- **Protocol**: MCP JSON-RPC over HTTP POST
- **Mode**: Mixed (per-server auto-detection)

**Key Operations**:
- `POST /mcp/{serverName}` - Path-based routing to specific MCP server (lines 49, 116-184)
- `POST /mcp` - Legacy routing with X-MCP-Server header (lines 50)
- `GET /health` - Health check with active server count (lines 51, 186-194)
- `GET /servers` - List available MCP servers from config (lines 52, 196-210)

**Startup** (lines 63-98):
- Initializes structured logging based on config level
- Logs "Mode: mixed (per-server)" in startup message
- Reports per-server mode detection (containers for docker.image, processes otherwise)

**Shutdown** (lines 100-113):
- `Stop(ctx)` method with context timeout
- Cancels background tasks via context
- Calls `ServerManager.StopAll()` to cleanup all instances
- Gracefully shuts down HTTP server

### 2. Server Manager (`internal/gateway/session.go`)
- **Responsibility**: Manage running MCP server instances
- **Pattern**: One persistent instance per server name
- **State**: Map of server name → ServerInstance
- **Lifecycle**: Create on first request, persist until shutdown
- **Mode Detection**: Per-server based on docker.image presence

**Key Methods**:
- `GetOrCreate(serverName, gateway)` - Returns existing or creates new instance
- `Get(serverName)` - Retrieve existing instance
- `Delete(serverName)` - Stop and remove instance
- `Count()` - Number of active instances
- `StopAll()` - Stop and cleanup all instances

**Instance Management**:
- Checks for stale instances (closed stdin/stdout)
- Automatically removes and recreates stale instances
- Thread-safe with RWMutex

### 3. Server Instance (`internal/gateway/session.go`)
- **Responsibility**: Represent a single running MCP server
- **Types**: Container (Docker) or Process (local subprocess)
- **Mode Detection**: Automatic per-server based on docker.image
- **Communication**: stdin/stdout pipes with JSON-RPC message framing
- **Persistence**: Lives until gateway shutdown or explicit stop

**Per-Server Mode Detection** (lines 97-106):
```go
// Determine spawn mode per-server: use container if Docker image is specified,
// otherwise use process mode
useContainer := serverConfig.Docker.Image != ""

log.Debug("Determined spawn mode", map[string]interface{}{
    "server_name":    serverName,
    "use_container":  useContainer,
    "docker_image":   serverConfig.Docker.Image,
})
```

**Fields**:
- `ServerName` - Unique identifier
- `ContainerID` - Docker container ID (if container mode)
- `ProcessID` - Process ID (if process mode)
- `stdin/stdout` - I/O pipes for communication
- `messageWriter/messageReader` - JSON-RPC message handlers
- `gateway` - Reference for cleanup operations

**Lifecycle Methods**:
- `SendMessage(ctx, body)` - Send JSON-RPC message and get response
- `Stop()` - Close pipes and remove container/kill process

### 4. Server Spawner (`internal/gateway/spawner.go`)
- **Responsibility**: Spawn MCP server containers or processes on-demand
- **Mode Detection**: Automatic per-server (docker.image present → container, otherwise → process)
- **Strategy**: One container/process per server name (persistent)

**Mode Selection Logic**:
- If `serverConfig.Docker.Image != ""` → spawn container
- Otherwise → spawn process
- No global mode setting required
- Each server can use different spawn mode

**Container Spawning** (`spawnContainer`):
1. Build container config from server definition
2. Remove any existing container with same name
3. Pull Docker image based on pull policy
4. Create container with `managed-by=mcp-manager` label
5. Start container
6. Attach to get stdin/stdout pipes
7. Initialize JSON-RPC message reader/writer

**Process Spawning** (`spawnProcess`):
1. Build command from server config
2. Set environment variables
3. Create stdin/stdout pipes
4. Start subprocess
5. Initialize JSON-RPC message reader/writer
6. Monitor process exit in goroutine

### 5. I/O Bridge & Protocol (`internal/gateway/protocol.go`)
- **Responsibility**: Handle JSON-RPC 2.0 message format
- **Framing**: Newline-delimited JSON messages
- **Client → Server**: Parse HTTP body → write to stdin with newline
- **Server → Client**: Read from stdout → parse → return as HTTP response

**Message Types**:
- Requests (with ID) - expect response
- Notifications (no ID) - no response expected
- Responses - match request ID

**Buffering**:
- Line-based buffering for clean message separation
- Handles Docker attach stream demultiplexing

### 6. Auto-Registration with Cleanup (`cmd/mcp-manager/main.go`)
- **Responsibility**: Automatically register/unregister servers with Claude Code
- **Timing**: Before gateway starts (in serve command)
- **Behavior**: Updates registration if config changed, removes stale servers, warns on failure but doesn't block startup

**Registration Function** (lines 81-174):
```go
func registerWithClaudeCode(cfg *config.Config, configPath string) error {
    // Get project path, executable path, config path
    // Load Claude Code configuration
    
    // Build set of valid servers from current config
    validServerNames := make(map[string]bool)
    validServerNames["mcp-manager"] = true
    for _, srv := range enabledServers {
        validServerNames[srv.Name] = true
    }
    
    // Clean up any servers that are no longer in our config
    existingServers := claudeConfig.ListServers(projectPath)
    gatewayURLPrefix := fmt.Sprintf("http://localhost:%d/mcp/", gatewayPort)
    
    for serverName, serverConfig := range existingServers {
        if !validServerNames[serverName] {
            // Check if this server was managed by mcp-manager
            isManagedServer := false
            
            if serverName == "mcp-manager" {
                isManagedServer = true
            } else if serverConfig.Type == "http" &&
                      serverConfig.URL != "" &&
                      len(serverConfig.URL) >= len(gatewayURLPrefix) &&
                      serverConfig.URL[:len(gatewayURLPrefix)] == gatewayURLPrefix {
                // HTTP servers using our gateway URL pattern
                isManagedServer = true
            }
            
            if isManagedServer {
                claudeConfig.UnregisterServer(projectPath, serverName)
            }
        }
    }
    
    // Register mcp-manager as stdio MCP server
    // Register each enabled server as HTTP endpoint through gateway
    // Save Claude Code configuration
}
```

**Cleanup Logic**:
- Identifies mcp-manager servers by:
  1. Name "mcp-manager" (the stdio server)
  2. Type "http" with URL matching `http://localhost:{port}/mcp/{name}` pattern
- Removes servers not in current config
- Preserves external servers (e.g., serena) that don't match pattern
- Ensures perfect sync between mcp-config.yaml and Claude Code config

**Auto-Registration in Serve Command** (lines 595-599):
```go
// Auto-register with Claude Code (updates registration if config changed)
if err := registerWithClaudeCode(cfg, configPath); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: Failed to auto-register with Claude Code: %v\n", err)
    // Continue anyway - registration failure shouldn't block startup
}
```

**Benefits**:
- Registration stays in sync with config changes
- Removed servers automatically unregistered
- No manual registration step needed
- Servers automatically available in Claude Code
- Non-blocking (warns on failure)
- External servers preserved

## Request Flow

### Initial Connection to Server
1. Client sends request to `POST /mcp/filesystem`
2. Gateway extracts server name from path ("filesystem")
3. Gateway calls `ServerManager.GetOrCreate("filesystem")`
4. Server Manager checks map, doesn't find instance
5. Creates new ServerInstance for "filesystem"
6. **Auto-detects mode**: Checks if `docker.image` is set → yes → container mode
7. Spawns Docker container with filesystem MCP server
8. Attaches to container stdin/stdout
9. Stores instance in map for future requests
10. Forwards message to server via stdin
11. Reads response from stdout
12. Returns response to client

### Subsequent Messages to Same Server
1. Client sends request to `POST /mcp/filesystem`
2. Gateway extracts server name ("filesystem")
3. Gateway calls `ServerManager.GetOrCreate("filesystem")`
4. Server Manager finds existing instance in map
5. Returns existing instance (no spawning)
6. Message forwarded to existing server
7. Response read and returned

### Mixed Mode Example
```yaml
servers:
  - name: filesystem
    docker:
      image: "mcp/filesystem:latest"  # Has docker.image → container mode
    ...
  
  - name: local-tool
    command: ["python3"]
    args: ["my_tool.py"]
    # No docker.image → process mode
```

Both servers run simultaneously through the same gateway.

## Cleanup Scenarios

### Scenario 1: Normal Shutdown (stdin close)
```
Claude Code disconnects
  ↓
serve command: serverErrChan receives EOF
  ↓
gateway.Stop(ctx) called
  ↓
ServerManager.StopAll()
  ↓
For each ServerInstance:
  - Close stdin/stdout pipes
  - If container: RemoveContainer(containerID, force=true)
  - If process: Kill process
  ↓
All servers stopped ✅
```

### Scenario 2: Signal Shutdown (SIGTERM/SIGINT)
```
User presses Ctrl+C or process killed
  ↓
serve command: sigChan receives signal
  ↓
mcpSrv.Stop() called
  ↓
gateway.Stop(ctx) called
  ↓
ServerManager.StopAll()
  ↓
For each ServerInstance:
  - Close stdin/stdout pipes
  - If container: RemoveContainer(containerID, force=true)
  - If process: Kill process
  ↓
All servers stopped ✅
```

### Scenario 3: Gateway Error
```
Gateway HTTP server error
  ↓
serve command: gwErrChan receives error
  ↓
context.Cancel() called
  ↓
ctx.Done() case in select
  ↓
mcpSrv.Stop() called
  ↓
gateway.Stop(ctx) called
  ↓
ServerManager.StopAll()
  ↓
All servers stopped ✅
```

### Scenario 4: Startup Cleanup
```
serve command starts
  ↓
cleanupStaleContainers(dockerClient) called
  ↓
docker.ListContainers(label=managed-by=mcp-manager)
  ↓
For each found container:
  - RemoveContainer(containerID, force=true)
  ↓
Safety net cleanup ✅
```

### Scenario 5: Auto-Sync Configuration Cleanup
```
serve command starts
  ↓
registerWithClaudeCode() called
  ↓
Load mcp-config.yaml (current servers)
  ↓
Load Claude Code config (registered servers)
  ↓
Build valid server set from config
  ↓
Scan registered servers for this project
  ↓
For each registered server:
  - In valid set? → Keep
  - Not in valid set AND mcp-manager pattern? → Remove
  - Not in valid set AND external pattern? → Keep (preserve)
  ↓
Register all valid servers
  ↓
Save Claude Code config
  ↓
Perfect sync ✅
```

## HTTP Transport Support (Added 2025-10-17)

The gateway now supports **dual transport types** - both stdio and HTTP-based MCP servers can run simultaneously.

### Transport Types

**Stdio Transport** (Original):
- Gateway spawns containers/processes
- Gateway attaches to stdin/stdout pipes
- Gateway bridges HTTP → stdio
- Examples: mcp/filesystem, mcp/git, sequential-thinking

**HTTP Transport** (Added 2025-10-17):
- Server runs as HTTP service (in container or external)
- Gateway forwards HTTP → HTTP
- Server may use Server-Sent Events (SSE) for responses
- Examples: context7 (documentation server)

### HTTP Transport Architecture

```
Client ←→ Gateway (HTTP) ←→ HTTP MCP Server
                            (container with port mapping
                             OR external HTTP endpoint)
```

### Transport Detection

Transport type is specified per-server in config:
```yaml
servers:
  - name: filesystem
    # No transport field = stdio (default)
    docker:
      image: mcp/filesystem:latest

  - name: context7
    transport: http  # Explicit HTTP transport
    port: 8080
    http_path: /mcp
    docker:
      image: lastmileai/context7:latest
```

**Detection Logic** (`internal/gateway/session.go:116-150`):
```go
// Determine transport type (default to stdio if not specified)
transport := serverConfig.Transport
if transport == "" {
    transport = "stdio"
}
instance.Transport = transport

// Determine spawn mode based on transport and configuration
useContainer := serverConfig.Docker.Image != ""

if transport == "http" {
    // HTTP transport
    if useContainer {
        // HTTP server in Docker container
        err = instance.spawnHTTPContainer(ctx, gateway, *serverConfig)
    } else {
        // External HTTP endpoint
        err = instance.connectHTTPExternal(ctx, *serverConfig)
    }
} else {
    // stdio transport
    if useContainer {
        err = instance.spawnContainer(ctx, gateway, *serverConfig)
    } else {
        err = instance.spawnProcess(ctx, gateway, *serverConfig)
    }
}
```

### HTTP Path Routing

**Problem**: Some HTTP-based MCP servers expose endpoints at specific paths (e.g., `/mcp` instead of `/`).

**Solution**: `http_path` configuration field for internal routing.

**Key Distinction**:
- **Client → Gateway**: Standard path (`/mcp/{serverName}`)
- **Gateway → Container**: Appends `http_path` if configured

**Example**:
```yaml
- name: context7
  transport: http
  port: 8080
  http_path: /mcp  # Internal routing only
```

**Client Request**: `POST http://localhost:52080/mcp/context7`
**Gateway Forwards To**: `POST http://localhost:{mapped_port}/mcp`

**Implementation**:
- `internal/gateway/spawner.go:415-420` - Append http_path to container URL
- `internal/claudecode/config.go:174-184` - Registration URL (no http_path)

### Transport-Aware Server Lifecycle

Server instance reuse must check transport-specific fields:

**HTTP Transport** - Check `httpClient` and `httpURL`:
```go
if instance.Transport == "http" {
    isUsable = instance.httpClient != nil && instance.httpURL != ""
}
```

**Stdio Transport** - Check `stdin` and `stdout`:
```go
else {
    isUsable = instance.stdin != nil && instance.stdout != nil
}
```

**Why This Matters**: Without transport-aware checks, HTTP servers would be marked "stale" (stdin/stdout are nil) and recreated on every request.

**Implementation**: `internal/gateway/session.go:59-87`

### HTTP-Specific Features

**1. Server-Sent Events (SSE) Support**

Some HTTP MCP servers return responses in SSE format:
```
event: message
data: {"jsonrpc":"2.0","id":1,"result":{...}}
```

**Requirements**:
- Accept header: `application/json, text/event-stream`
- Accept HTTP 202 Accepted status (not just 200)
- Parse SSE format to extract JSON from `data:` line

**Implementation** (`internal/gateway/session.go:315-394`):
```go
// Set headers for JSON-RPC
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json, text/event-stream")

// Check status code - accept any 2xx status
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
}

// Check if response is SSE format
if strings.HasPrefix(responseStr, "event:") {
    // Parse SSE format - extract JSON from "data: " line
    lines := strings.Split(responseStr, "\n")
    for _, line := range lines {
        if strings.HasPrefix(line, "data: ") {
            jsonData := strings.TrimPrefix(line, "data: ")
            responseData = []byte(jsonData)
            break
        }
    }
}
```

**2. HTTP Status Code Handling**

Gateway accepts all 2xx status codes, not just 200:
- 200 OK - Standard success
- 202 Accepted - Common for SSE responses
- 204 No Content - Valid success response

**3. Container Port Mapping**

HTTP containers require port mapping:
```go
// Add port mapping for HTTP server
containerConfig.PortMappings = []string{fmt.Sprintf("0:%d", serverConfig.Port)}

// Get the mapped port after starting
mappedPort, err := gateway.docker.GetMappedPort(ctx, containerID, serverConfig.Port)

// Build HTTP URL with mapped port
s.httpURL = fmt.Sprintf("http://localhost:%d", mappedPort)
if serverConfig.HTTPPath != "" {
    s.httpURL = s.httpURL + serverConfig.HTTPPath
}
```

**Implementation**: `internal/gateway/spawner.go:317-468`

**4. HTTP Container Readiness Check** (Added 2025-10-17)

**Problem**: Containers start immediately but HTTP servers inside take time to initialize, causing first connection to fail.

**Solution**: TCP port polling with retry logic before returning ServerInstance.

```go
// Wait for HTTP server to be ready before returning
maxAttempts := 30 // 30 seconds max wait time
for i := 0; i < maxAttempts; i++ {
    // Try to connect to the port
    conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", mappedPort), 1*time.Second)
    if err == nil {
        conn.Close()
        log.Info("HTTP server ready", map[string]interface{}{
            "server_name": serverConfig.Name,
            "attempt":     i + 1,
        })
        break
    }

    if i == maxAttempts-1 {
        // Timeout - cleanup and fail
        gateway.docker.RemoveContainer(ctx, containerID, true)
        return fmt.Errorf("HTTP server did not become ready after %d seconds", maxAttempts)
    }

    time.Sleep(1 * time.Second)
}
```

**Characteristics**:
- Polls TCP port every second for up to 30 seconds
- Uses simple TCP connection test (agnostic to HTTP specifics)
- Cleans up container if server never becomes ready
- Only stores ServerInstance when server is actually accepting connections
- Eliminates first-connection failures seen by users

**Implementation**: `internal/gateway/spawner.go:425-458`

**5. External HTTP Endpoints**

Support for external HTTP-based MCP servers (no Docker):
```yaml
- name: external-api
  enabled: true
  transport: http
  url: https://api.example.com/mcp
```

**Implementation**: `internal/gateway/spawner.go:435-453`

### Message Routing by Transport

**Stdio Transport** (`internal/gateway/session.go:239-313`):
1. Parse JSON-RPC request
2. Detect notifications (no ID field)
3. Write to server stdin with newline
4. Read response from stdout (skip for notifications)
5. Serialize and return response

**HTTP Transport** (`internal/gateway/session.go:315-394`):
1. Create HTTP POST request with JSON body
2. Set required headers (Content-Type, Accept)
3. Send to server's HTTP endpoint
4. Check status code (accept 2xx)
5. Parse SSE format if present
6. Return JSON response

### JSON-RPC 2.0 Notification Handling

**Critical for HTTP Transport**: Claude Code sends `notifications/initialized` which must not receive a response.

**Detection**: Check if `msg.ID == nil`

**Implementation** (`internal/mcpserver/server.go:74-98`):
```go
// Check if this is a notification (no ID field)
isNotification := msg.ID == nil

switch msg.Method {
case "notifications/initialized":
    // No response needed for notifications
    return nil
default:
    // If it's a notification, don't respond
    if isNotification {
        return nil
    }
    return s.createErrorResponse(msg.ID, -32601, "Method not found", "")
}
```

**Impact**: Prevents schema validation errors in clients when notifications are incorrectly answered.

### HTTP Transport Configuration Options

**Container-based HTTP Server**:
```yaml
- name: context7
  enabled: true
  transport: http
  port: 8080           # Container's internal port
  http_path: /mcp      # Optional: path to MCP endpoint
  docker:
    image: lastmileai/context7:latest
    pull_policy: if_not_present
  env:
    - name: CONTEXT7_API_KEY
      value_from_env: CONTEXT7_API_KEY
```

**External HTTP Server** (no Docker):
```yaml
- name: external-service
  enabled: true
  transport: http
  url: https://api.example.com/mcp
```

### Mixed Transport Example

Both transport types work simultaneously:

```yaml
servers:
  # Stdio transport (default)
  - name: filesystem
    enabled: true
    docker:
      image: mcp/filesystem:latest
    volumes:
      - host_path: ${PWD}
        container_path: /workspace

  # Stdio transport (explicit)
  - name: sequential-thinking
    enabled: true
    transport: stdio
    docker:
      image: ghcr.io/lastmileai/mcp-sequential-thinking:latest

  # HTTP transport (container)
  - name: context7
    enabled: true
    transport: http
    port: 8080
    http_path: /mcp
    docker:
      image: lastmileai/context7:latest
    env:
      - name: CONTEXT7_API_KEY
        value_from_env: CONTEXT7_API_KEY

  # HTTP transport (external)
  - name: remote-api
    enabled: true
    transport: http
    url: https://api.example.com/mcp
```

All four servers run through the same gateway instance.

### Testing (2025-10-17)

Successfully tested with:
- **context7** (HTTP/SSE transport in Docker): ✅ Connected
- **sequential-thinking** (stdio transport in Docker): ✅ Connected
- **Both simultaneously through gateway**: ✅ Working
- **HTTP container readiness check**: ✅ First connection succeeds immediately

**Verified**:
- Transport-aware server reuse
- SSE response parsing
- HTTP status code handling (202 Accepted)
- Accept header requirements
- JSON-RPC notification handling
- http_path routing
- **HTTP port health check** - Eliminates first-connection failures
- **Startup time absorption** - Gateway waits for container to be ready

### Related Files (HTTP Transport)

**Core Implementation**:
- `internal/gateway/session.go:59-87` - Transport-aware reuse logic
- `internal/gateway/session.go:206-237` - Transport routing
- `internal/gateway/session.go:239-313` - Stdio message handler
- `internal/gateway/session.go:315-394` - HTTP message handler
- `internal/gateway/spawner.go:317-468` - HTTP container spawning with health check
- `internal/gateway/spawner.go:425-458` - HTTP readiness check loop
- `internal/gateway/spawner.go:470-488` - External HTTP connection
- `internal/mcpserver/server.go:74-98` - Notification handling
- `internal/claudecode/config.go:174-184` - Registration URL generation

**Configuration**:
- `internal/config/config.go` - Added Transport, Port, HTTPPath, URL fields
- `mcp-config.yaml` - Added http_path configuration

### Advantages of Dual Transport

1. **Universal MCP Server Support**: Works with ANY MCP server regardless of transport
2. **Flexibility**: Mix stdio and HTTP servers as needed
3. **Performance**: HTTP servers can be optimized for network communication
4. **Compatibility**: Stdio servers work as before (no breaking changes)
5. **External Integration**: Can connect to remote HTTP-based MCP services
6. **Standard Protocols**: HTTP transport uses standard web protocols

## Configuration

### Gateway Config (`mcp-config.yaml`)
```yaml
settings:
  gateway_port: 52080
  network: mcp-network
  data_dir: ~/.mcp-manager
  log_level: info
  # gateway_mode is deprecated - gateway now automatically determines mode per-server
  # Servers with docker.image run as containers, others run as processes

servers:
  # Container mode (has docker.image)
  - name: filesystem
    enabled: true
    description: "Filesystem access capabilities"
    docker:
      image: "mcp/filesystem:latest"
      pull_policy: if-not-present
    command: []
    args: []
    volumes:
      - host_path: ${PWD}
        container_path: /workspace
        read_only: false
    agents:
      - claude-code
  
  # Process mode (no docker.image)
  - name: local-tool
    enabled: true
    description: "Local Python tool"
    command: ["python3"]
    args: ["my_tool.py"]
    agents:
      - claude-code
```

**Key Changes from Previous Version**:
- `gateway_mode` setting removed (now per-server auto-detection)
- Servers can mix container and process modes
- Mode determined by presence of `docker.image` field
- Auto-sync cleanup removes stale servers

## Container Labeling

All Docker containers spawned by the gateway are labeled:
```json
{
  "managed-by": "mcp-manager",
  "server": "filesystem"  // server name
}
```

This enables:
- Identification of mcp-manager containers
- Cleanup of stale containers on startup
- Filtering in `docker ps` commands

## Implementation Status

### ✅ Completed Features
- [x] HTTP server with MCP endpoints
- [x] Server instance management (persistent)
- [x] Container spawning with Docker attach
- [x] Process spawning with local commands
- [x] **Per-server mode auto-detection** (2025-10-16)
- [x] **Mixed mode gateway** (2025-10-16)
- [x] JSON-RPC 2.0 protocol handling
- [x] Path-based routing (/mcp/{server})
- [x] Graceful shutdown with cleanup
- [x] Signal handling (SIGTERM, SIGINT)
- [x] Startup cleanup of stale containers
- [x] Container labeling for identification
- [x] Health and server list endpoints
- [x] **Auto-registration on startup** (2025-10-16)
- [x] **Auto-sync configuration cleanup** (2025-10-16)

### 🚧 In Progress
- [ ] Structured logging (partially done with internal/log package)
- [ ] Metrics and monitoring

### 📋 Planned
- [ ] SSE streaming support
- [ ] Connection pooling
- [ ] Health monitoring for containers

## Advantages Over Direct Container Communication

1. **Clean Lifecycle**: Gateway manages container lifecycle, ensures cleanup
2. **Protocol Bridging**: HTTP ↔ stdio translation
3. **Session Management**: One instance per server, persistent
4. **Flexibility**: Mix container and process mode servers
5. **Auto-Detection**: No manual mode configuration needed
6. **Auto-Sync**: Config changes automatically reflected in Claude Code
7. **Independence**: No reliance on Docker Inc. infrastructure
8. **Simplicity**: Single gateway binary, clear routing
9. **Auto-Registration**: Stays in sync with config changes
10. **Extensibility**: Easy to add features (logging, metrics, etc.)

## Key Differences from Original Design

**Original (Per-Client Sessions)**:
- New container per client session
- Session timeouts with cleanup
- Per-session isolation

**Current (Persistent Servers)**:
- One container/process per server name
- Persists until gateway shutdown
- Shared server instance for all clients

**Previous (Global Mode)**:
- `gateway_mode: container` or `gateway_mode: process`
- All servers used same mode
- Manual configuration required

**Current (Per-Server Auto-Detection)**:
- No global mode setting
- Each server automatically selects container or process mode
- Based on presence of `docker.image` field
- Servers can mix modes

**Previous (Manual Sync)**:
- Manual registration required after config changes
- Stale servers remained in Claude Code config
- Manual unregistration needed

**Current (Auto-Sync)**:
- Automatic registration on startup
- Automatic cleanup of stale servers
- Pattern-based server identification
- Preserves external servers

**Rationale for Changes**:
- MCP servers maintain state (e.g., tool registrations)
- Client expects consistent server across requests
- Reduces container churn
- Better resource efficiency
- Simpler lifecycle management
- More flexible deployment options (mix local and containerized servers)
- Zero-friction registration workflow
- Perfect config sync

## Error Handling

**Container Spawn Failures**:
- Pull errors → return error to client
- Create errors → cleanup and return error
- Start errors → cleanup and return error
- Attach errors → cleanup and return error

**Process Spawn Failures**:
- Command not found → return error to client
- Start errors → return error to client

**Runtime Errors**:
- Stale instance detected → remove and recreate
- Message send errors → return error to client
- Message read errors → return error to client

**Shutdown Errors**:
- Container removal errors → log but continue
- Process kill errors → log but continue
- Best-effort cleanup
- Startup cleanup catches any missed containers

**Registration Errors**:
- Auto-registration failures → warn but don't block startup
- Allows gateway to start even if Claude Code not available

**Auto-Sync Errors**:
- Pattern matching failures → log and skip server
- External server detection failures → preserve server (safe default)
- Claude Code config save failures → warn but continue

## Performance Considerations

**Container Reuse**: 
- First request spawns container (~2-5 seconds)
- Subsequent requests use existing container (~50ms)

**Process Spawn**:
- First request spawns process (~100-500ms)
- Subsequent requests use existing process (~10-50ms)

**Auto-Sync Overhead**:
- Pattern matching: ~10ms per registered server
- Config save: ~20-50ms
- Total auto-sync overhead: ~50-100ms on startup

**Memory**:
- Gateway overhead: ~10-20 MB
- Per-container overhead: ~30-100 MB (depends on server)
- Per-process overhead: ~10-50 MB (depends on server)

**Startup Time**:
- Gateway starts in ~100ms
- Auto-registration adds ~50-100ms
- Auto-sync cleanup adds ~10ms
- Container spawn adds 2-5 seconds per server
- Process spawn adds 100-500ms per server
- On-demand spawning reduces initial startup time

## Testing

**Manual Tests**:
```bash
# Start gateway (with auto-registration and cleanup)
./mcp-manager serve --config mcp-config.yaml

# In another terminal
# Check containers before request
docker ps --filter label=managed-by=mcp-manager

# Send MCP request (triggers spawn)
curl -X POST http://localhost:52080/mcp/filesystem \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

# Check containers after request (should see filesystem container)
docker ps --filter label=managed-by=mcp-manager

# Stop gateway with Ctrl+C
# Check containers again (should be empty)
docker ps -a --filter label=managed-by=mcp-manager

# Verify registration
cat ~/.claude.json | jq '.projects["/path/to/project"].mcpServers'

# Test auto-sync cleanup
# Remove a server from mcp-config.yaml
# Restart serve
# Verify server was auto-removed from Claude Code config
```

**Integration Tests**:
- Located in `test/` directory
- Test both container and process modes (lines 45-49 in test/integration_test.go)
- Test cleanup scenarios
- Test signal handling

## Related Files

**Core Gateway**:
- `internal/gateway/gateway.go:14-31` - Gateway struct (no mode field)
- `internal/gateway/gateway.go:81-91` - Startup logging (mixed mode)
- `internal/gateway/gateway.go:116-184` - MCP request handler
- `internal/gateway/session.go:97-106` - Per-server mode detection
- `internal/gateway/spawner.go` - Container/process spawning
- `internal/gateway/protocol.go` - JSON-RPC message handling

**CLI Integration**:
- `cmd/mcp-manager/main.go:81-174` - Auto-registration function with cleanup
- `cmd/mcp-manager/main.go:124-151` - Auto-sync cleanup logic
- `cmd/mcp-manager/main.go:595-599` - Auto-registration call in serve
- `cmd/mcp-manager/main.go:713-756` - Register command (uses helper)
- `cmd/mcp-manager/main.go:414-486` - `gateway` command (standalone)
- `cmd/mcp-manager/main.go:488-615` - `serve` command (with signal handling)

**Docker Client**:
- `internal/docker/client.go` - Docker API wrapper
- Methods: CreateContainer, StartContainer, StopContainer, RemoveContainer
- Method: AttachContainer (for stdin/stdout pipes)
- Method: ListContainers (for cleanup)

**Configuration**:
- `internal/config/config.go` - Config structs and validation
- `mcp-config.yaml:8-14` - Settings with deprecated gateway_mode comment

**Logging**:
- `internal/log/` - Structured logging package (new)

**Tests**:
- `test/integration_test.go:45-49` - Gateway config without mode
- `internal/config/config_test.go` - Config validation tests
- `internal/gateway/protocol_test.go` - Protocol tests
- `internal/mcpserver/server_test.go` - Server tests