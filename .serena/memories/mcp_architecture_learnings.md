# MCP Architecture Learnings

## The Stdio Problem

### What We Discovered
MCP servers in the ecosystem (like `mcp/filesystem`, `mcp/git`) use **stdio transport**:
- Designed to run as subprocesses spawned by clients
- Communicate via stdin/stdout using JSON-RPC
- Exit immediately when stdin closes (no input = exit)
- **NOT designed as long-running daemons**

### Why Docker Containers Loop/Crash
When we tried running stdio MCP servers as persistent Docker containers:
1. Container starts → MCP server runs → waits for stdin
2. No stdin provided → EOF on stdin → server exits (clean exit code 0)
3. Restart policy (`unless-stopped`) → Docker restarts container
4. Loop repeats infinitely (observed 362+ restarts in minutes)

This is a **fundamental architectural mismatch**.

## MCP Transport Options

### 1. STDIO (Standard Input/Output)
- **Use case**: Local subprocess communication
- **Pros**: Most common, works with existing servers
- **Cons**: Requires process spawning, not suitable for persistent containers
- **Examples**: Most MCP servers (`mcp/filesystem`, `mcp/git`, etc.)

### 2. Streamable HTTP (Modern Standard)
- **Use case**: Network-based, Docker-friendly
- **Pros**: HTTP/HTTPS, supports streaming, works as daemon
- **Cons**: Fewer servers support it currently
- **Protocol**: HTTP POST for client→server, optional SSE for server→client

### 3. SSE (Deprecated)
- Legacy transport, replaced by Streamable HTTP

## Docker's Solution: MCP Gateway

### Architecture
```
AI Client ←→ MCP Gateway ←→ MCP Server Containers (stdio)
           (HTTP/Streaming)     (spawned on-demand)
```

### How It Works
1. **Gateway runs as persistent service**
   - Listens on HTTP/streaming transport (e.g., port 8080)
   - Clients connect to gateway, not directly to MCP servers

2. **On-Demand Container Spawning**
   - Containers are **NOT** kept running continuously
   - Gateway spawns stdio MCP server containers when needed
   - Gateway pipes stdin/stdout to communicate with containers
   - Containers exit when request completes

3. **Protocol Translation**
   - Client ←→ Gateway: HTTP/Streaming transport
   - Gateway ←→ Server: stdio (subprocess/container)
   - Gateway handles routing and multiplexing

### Key Features
- Container isolation with minimal privileges
- Dynamic tool discovery
- Secure secrets/OAuth management
- Configuration in `~/.docker/mcp/`

## Implications for mcp-manager

### Current Approach (Broken)
- ❌ Tries to run stdio servers as persistent containers
- ❌ Uses restart policies causing infinite loops
- ❌ No protocol translation layer

### Options Forward

**Option 1: Integrate with Docker MCP Gateway**
- Use Docker's gateway as the backend
- mcp-manager becomes a configuration management tool
- Manages `~/.docker/mcp/` configuration files
- Gateway handles container lifecycle

**Option 2: Build Our Own Gateway**
- Implement HTTP/streaming endpoint
- Spawn stdio containers on-demand (no restart policy)
- Handle stdin/stdout piping
- More control, more complexity

**Option 3: Pivot to Process Management**
- Don't use Docker at all
- Manage stdio MCP servers as local processes
- Direct subprocess spawning like Claude Code does
- Simpler but loses container isolation

**Option 4: HTTP-Native Servers Only**
- Only support MCP servers with HTTP transport
- Servers run as persistent containers (like normal web services)
- Limited ecosystem support currently

## Recommended Path

Given the ecosystem is primarily stdio-based and Docker has already solved this, **Option 1** (integrate with Docker MCP Gateway) makes the most sense:

1. mcp-manager focuses on **configuration management**
2. Use Docker MCP Gateway for actual server runtime
3. Provide better UX around gateway configuration
4. Add features Docker gateway doesn't have (monitoring, updates, etc.)

Alternatively, **Option 2** if we want full control and custom features, but requires implementing the gateway pattern ourselves.
