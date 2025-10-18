# Current Configuration

Last Updated: 2025-10-16

## Active Configuration File

**Primary**: `mcp-config.yaml` (project root)

## Gateway Settings

```yaml
settings:
  data_dir: ~/.mcp-manager
  network: mcp-network
  log_level: debug
  gateway_port: 52080
  # gateway_mode is deprecated - gateway now automatically determines mode per-server
  # Servers with docker.image run as containers, others run as processes
```

**Gateway Details**:
- Mode: Mixed (per-server auto-detection)
  - Servers with `docker.image` → run as containers
  - Servers without `docker.image` → run as processes
- Port: 52080 (configurable)
- Persistent servers (one instance per server name)
- Endpoints:
  - `/health` - Gateway health check
  - `/servers` - List available MCP servers
  - `/mcp/{server-name}` - MCP protocol endpoint for specific server
- Auto-registration: Enabled (registers with Claude Code on startup)
- Auto-cleanup: Enabled (removes stale servers from Claude Code config)

## Configured MCP Servers

All servers from Docker Hub, running in container mode (all have `docker.image`):

### 1. Git Server
```yaml
name: git
enabled: true
description: "Git repository access and operations"
docker:
  image: "mcp/git:latest"
  pull_policy: if-not-present
command: []  # Uses image's ENTRYPOINT/CMD
args: []
env:
  - name: NODE_ENV
    value: production
volumes:
  - host_path: /Users/ben.gittins/Code/work/mcp-manager
    container_path: /workspace
    read_only: false
health_check:
  enabled: false
agents:
  - claude-code
```

**Mode**: Container (has docker.image)  
**Claude Code URL**: `http://localhost:52080/mcp/git`

### 2. Sequential Thinking Server
```yaml
name: sequential-thinking
enabled: true
description: "Structured reasoning and thinking capabilities"
docker:
  image: "mcp/sequentialthinking"
  pull_policy: if-not-present
command: []  # Uses image's ENTRYPOINT/CMD
args: []
env:
  - name: NODE_ENV
    value: production
volumes: []
health_check:
  enabled: false
agents:
  - claude-code
```

**Mode**: Container (has docker.image)  
**Claude Code URL**: `http://localhost:52080/mcp/sequential-thinking`

### Removed Servers

**Filesystem** (removed 2025-10-16):
- Previously configured with `mcp/filesystem:latest`
- Removed from `mcp-config.yaml`
- Automatically unregistered from Claude Code via auto-sync cleanup ✅
- No manual cleanup required

### Example: Process Mode Server (Not Currently Configured)
```yaml
name: local-tool
enabled: true
description: "Local Python tool"
command: ["python3"]
args: ["my_tool.py"]
# No docker.image → will run as process
agents:
  - claude-code
```

**Mode**: Process (no docker.image)  
**Claude Code URL**: `http://localhost:52080/mcp/local-tool`

## Claude Code Registration

**Status**: Auto-registered ✅

**Registration Timing**: 
- Automatically runs when `mcp-manager serve` starts
- Updates registration if config changed
- **Removes stale servers** that are no longer in config
- Preserves external servers (e.g., serena)
- Fails gracefully (warns but doesn't block startup)

**Manual Registration** (optional):
```bash
./mcp-manager register --config mcp-config.yaml
```

**Registered Servers in ~/.claude.json**:

1. **mcp-manager** (stdio)
   - Type: stdio
   - Command: `/Users/ben.gittins/Code/work/mcp-manager/mcp-manager`
   - Args: `["serve", "--config", "/Users/ben.gittins/Code/work/mcp-manager/mcp-config.yaml"]`
   - Provides: Management tools (list_servers, server_status, gateway_status)

2. **git** (http)
   - Type: http
   - URL: `http://localhost:52080/mcp/git`
   - Provides: Git tools (status, log, diff, commit, etc.)

3. **sequential-thinking** (http)
   - Type: http
   - URL: `http://localhost:52080/mcp/sequential-thinking`
   - Provides: Thinking tools (structured reasoning)

4. **serena** (stdio) - Separate registration (external, preserved)
   - Type: stdio
   - Command: `uvx`
   - Args: `["--from", "git+https://github.com/oraios/serena", "serena", "start-mcp-server", "--context", "ide-assistant", "--project", "/Users/ben.gittins/Code/work/mcp-manager"]`
   - Provides: Code navigation and semantic tools
   - **Not managed by mcp-manager** - preserved during auto-sync cleanup

## Auto-Sync Configuration Cleanup

**Feature**: Automatically removes servers from Claude Code when they're removed from mcp-config.yaml

**How It Works**:
1. When `serve` starts, it calls `registerWithClaudeCode()`
2. Function builds list of valid servers from current config
3. Scans existing Claude Code registration for this project
4. Identifies mcp-manager servers by:
   - Name "mcp-manager" (the stdio server)
   - Type "http" with URL pattern `http://localhost:{port}/mcp/{name}`
5. Removes any mcp-manager servers not in valid list
6. Preserves external servers (like serena) that don't match pattern
7. Registers all current enabled servers

**Example**:
```bash
# Before: filesystem, git, sequential-thinking registered
# Remove filesystem from mcp-config.yaml
# Run: ./mcp-manager serve
# After: git, sequential-thinking registered (filesystem auto-removed)
```

**Benefits**:
- No manual unregistration needed
- Config stays in perfect sync
- External servers preserved
- Clean, automated workflow

## How It Works

### Startup Flow
```
1. Claude Code starts
2. Launches: mcp-manager serve --config mcp-config.yaml
3. mcp-manager runs auto-sync:
   a. Identifies stale mcp-manager servers in Claude Code
   b. Removes stale servers
   c. Registers current enabled servers
4. mcp-manager starts gateway on port 52080
5. Gateway listens for HTTP requests in mixed mode
6. Claude Code connects to HTTP endpoints for each server
```

### Auto-Sync Cleanup Flow
```
1. Load mcp-config.yaml (e.g., git, sequential-thinking)
2. Load Claude Code config (~/.claude.json)
3. Build valid server set: {mcp-manager, git, sequential-thinking}
4. Scan Claude Code servers for this project
5. For each registered server:
   - Check if in valid set → keep
   - Not in valid set AND matches pattern → remove (e.g., filesystem)
   - Not in valid set AND doesn't match pattern → keep (e.g., serena)
6. Register all valid servers
7. Save Claude Code config
```

### Request Flow (e.g., git tools)
```
1. Claude Code → HTTP POST to http://localhost:52080/mcp/git
2. Gateway extracts server name from path ("git")
3. Gateway calls ServerManager.GetOrCreate("git")
4. Server Manager checks map, doesn't find instance
5. Creates new ServerInstance for "git"
6. Auto-detects mode: git has docker.image → container mode
7. Spawns Docker container with git MCP server
8. Attaches to container stdin/stdout
9. Stores instance in map (persistent)
10. Forwards JSON-RPC message via stdin
11. Reads JSON-RPC response from stdout
12. Returns response to Claude Code
13. Instance persists for future requests (no timeout)
```

### Subsequent Messages to Same Server
```
1. Claude Code → HTTP POST to http://localhost:52080/mcp/git
2. Gateway extracts server name ("git")
3. Gateway calls ServerManager.GetOrCreate("git")
4. Server Manager finds existing instance in map
5. Returns existing instance (no spawning)
6. Message forwarded to existing server via stdin
7. Response read from stdout and returned
```

### Mixed Mode Behavior
The gateway can simultaneously run:
- Container-based servers (e.g., git, sequential-thinking)
- Process-based servers (e.g., local Python/Node scripts)

Mode is determined per-server automatically:
- If `docker.image` is present → container mode
- If `docker.image` is absent → process mode

### Architecture Benefits
- **Single gateway process**: One persistent gateway for all servers
- **On-demand spawning**: Servers start only when first requested
- **Persistent instances**: Servers stay running until gateway shutdown
- **Per-server mode detection**: No manual mode configuration
- **Flexible deployment**: Mix containerized and local servers
- **Proper namespacing**: Each server's tools maintain their namespace
- **Resource efficiency**: Clean container/process lifecycle management
- **Auto-registration**: Stays in sync with config changes
- **Auto-cleanup**: Removes stale servers automatically

## Configuration File Locations

Searches in order:
1. `--config` flag path
2. `./mcp-config.yaml`
3. `./.mcp-manager/config.yaml`
4. `~/.mcp-manager/config.yaml`

## Common Commands

**Registration**:
```bash
# Manual registration with cleanup (usually not needed due to auto-registration)
./mcp-manager register --config mcp-config.yaml --verbose

# Unregister all mcp-manager servers
./mcp-manager unregister

# Verify registration
cat ~/.claude.json | jq '.projects["/Users/ben.gittins/Code/work/mcp-manager"].mcpServers'
```

**Gateway**:
```bash
# Start as MCP server with gateway (recommended for Claude Code)
# Auto-registers and cleans up on startup
./mcp-manager serve --config mcp-config.yaml

# Start standalone gateway (no auto-registration/cleanup)
./mcp-manager gateway --port 52080
```

**Validation**:
```bash
# Validate config
./mcp-manager validate --config mcp-config.yaml

# List servers
./mcp-manager list --config mcp-config.yaml
```

**MCP Manager Tools** (via Claude Code):
```bash
# Check gateway status
gateway_status

# List configured servers
list_servers

# Check specific server status
server_status --server git
```

## Recent Changes

### 2025-10-16: Auto-Sync Configuration Cleanup
**Change**: Added automatic cleanup of stale servers from Claude Code

**Impact**:
- Removing servers from mcp-config.yaml automatically unregisters them
- External servers (serena) preserved during cleanup
- Pattern-based identification of mcp-manager servers
- Perfect config sync between mcp-config.yaml and Claude Code

**Files Changed**:
- `cmd/mcp-manager/main.go:124-151` - Cleanup logic
- Identifies mcp-manager servers by URL pattern or name
- Preserves external servers that don't match pattern

**Migration**:
- Old: Manual unregistration required after removing servers
- New: Automatic unregistration on next `serve` start

### 2025-10-16: Mixed Mode Gateway
**Change**: Removed global `gateway_mode` setting, implemented per-server auto-detection

**Impact**:
- Gateway now supports both container and process servers simultaneously
- Mode determined automatically based on presence of `docker.image` field
- No manual mode configuration required
- More flexible deployment options

**Files Changed**:
- `internal/gateway/gateway.go` - Removed mode field from Gateway struct
- `internal/gateway/session.go` - Per-server mode detection logic
- `internal/mcpserver/server.go` - Updated gateway_status tool output
- `cmd/mcp-manager/main.go` - Removed mode parameter
- `test/integration_test.go` - Updated test config
- `mcp-config.yaml` - Replaced gateway_mode with deprecation comment

**Migration**: 
- Old: `gateway_mode: container` in settings
- New: Automatic detection per-server (no setting needed)

### 2025-10-16: Auto-Registration on Startup
**Change**: Gateway now automatically registers with Claude Code when starting

**Impact**:
- `mcp-manager serve` automatically updates Claude Code registration
- Registration stays in sync with config changes
- Manual registration usually not needed
- Fails gracefully (warns but doesn't block startup)

**Files Changed**:
- `cmd/mcp-manager/main.go` - New `registerWithClaudeCode()` helper function
- `cmd/mcp-manager/main.go` - Auto-registration call in serve command
- `cmd/mcp-manager/main.go` - Simplified register command to use helper

**Behavior**:
- Runs before gateway starts
- Updates both mcp-manager and all enabled servers
- Non-blocking (warnings only on failure)

### 2025-10-15: Registration Fix
**Change**: Fixed registration to include all enabled servers

**Impact**:
- Previously: Only `mcp-manager` was registered
- Now: `mcp-manager` + all enabled servers are registered
- Individual MCP servers now accessible in Claude Code

**Files Changed**: 
- `cmd/mcp-manager/main.go` (register/unregister commands)

### 2025-10-15: Container Lifecycle Management
**Change**: Implemented proper container cleanup

**Impact**:
- Containers cleaned up on gateway shutdown
- Startup cleanup removes stale containers
- Signal handling (SIGTERM/SIGINT) triggers cleanup
- No orphaned containers

**Files Changed**: 
- `internal/gateway/gateway.go` - Stop method
- `internal/gateway/session.go` - ServerManager.StopAll
- `cmd/mcp-manager/main.go` - Signal handling and cleanup

## Docker Images Used

All from Docker Hub:
- `mcp/git:latest`
- `mcp/sequentialthinking` (also known as sequential-thinking)

Note: Can also use Docker-based MCPs with empty command/args - they will use the image's ENTRYPOINT/CMD.

## Pull Policies

Configured with `pull_policy: if-not-present`:
- Automatically pulls image if not present locally
- Skips pull if image exists locally
- Other options: `always`, `never`

## Environment Variables

Servers can use environment variables:
```yaml
env:
  - name: NODE_ENV
    value: production
  - name: FROM_HOST
    value_from_env: HOST_VAR_NAME  # Gets value from host environment
```

## Volume Mounts

Git server mounts the project directory:
```yaml
volumes:
  - host_path: /Users/ben.gittins/Code/work/mcp-manager
    container_path: /workspace
    read_only: false
```

Sequential thinking server has no volumes (doesn't need filesystem access).

## Health Checks

Currently disabled for all servers:
```yaml
health_check:
  enabled: false
```

## Gateway Endpoints

**Health Check**:
```bash
curl http://localhost:52080/health
# Returns: {"status":"healthy","active_servers":2}
```

**Server List**:
```bash
curl http://localhost:52080/servers
# Returns: [{"name":"git","description":"...","image":"mcp/git:latest"},...]
```

**MCP Protocol** (example with git):
```bash
curl -X POST http://localhost:52080/mcp/git \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

## Logging

**Log Level**: `debug` (in mcp-config.yaml)

**Log Destinations**:
- Structured logs: JSON format to stderr (via internal/log package)
- User-facing logs: Plain text to stdout

**Log Output Examples**:
```json
{
  "timestamp": "2025-10-16T09:26:07.258093+10:00",
  "level": "INFO",
  "message": "MCP Gateway starting",
  "fields": {
    "host": "127.0.0.1",
    "port": 52080,
    "mode": "mixed (per-server)",
    "available_servers": 2,
    "log_level": "debug"
  }
}
```

**Available Levels**: `debug`, `info`, `warn`, `error`

## Testing the Configuration

### 1. Verify Configuration
```bash
./mcp-manager validate --config mcp-config.yaml
```

### 2. List Enabled Servers
```bash
./mcp-manager list --config mcp-config.yaml
```

### 3. Start Gateway
```bash
./mcp-manager serve --config mcp-config.yaml
# Should see:
# - Auto-registration output (with cleanup)
# - Gateway starting on 127.0.0.1:52080
# - Mode: mixed (per-server)
# - Available servers: 2
```

### 4. Check Containers (After First Request)
```bash
# No containers initially (on-demand spawning)
docker ps --filter label=managed-by=mcp-manager

# After Claude Code makes a request to git server
docker ps --filter label=managed-by=mcp-manager
# Should show: git container
```

### 5. Test with Claude Code
```bash
# In Claude Code:
/mcp list_servers         # List all registered servers
/mcp gateway_status       # Check gateway status
/mcp git__git_status      # Use git MCP (triggers container spawn)
```

### 6. Verify Cleanup
```bash
# Stop gateway (Ctrl+C)
# Check containers are removed
docker ps -a --filter label=managed-by=mcp-manager
# Should show: empty list
```

### 7. Test Auto-Sync Cleanup
```bash
# Check current registration
cat ~/.claude.json | jq '.projects["/path/to/project"].mcpServers | keys'
# Should show: ["git", "mcp-manager", "sequential-thinking", "serena"]

# Remove a server from mcp-config.yaml (e.g., git)
# Restart serve
./mcp-manager serve --config mcp-config.yaml

# Check registration again
cat ~/.claude.json | jq '.projects["/path/to/project"].mcpServers | keys'
# Should show: ["mcp-manager", "sequential-thinking", "serena"] (git removed)
```

## Troubleshooting

### Gateway Not Starting
```bash
# Check if port is already in use
lsof -i :52080

# Kill existing process
pkill -f "mcp-manager serve"
```

### Container Fails to Start
```bash
# Check Docker daemon
docker ps

# Check container logs
docker logs <container-id>

# Manual cleanup
docker rm -f $(docker ps -aq --filter label=managed-by=mcp-manager)
```

### Registration Issues
```bash
# Check Claude Code config
cat ~/.claude.json | jq '.projects'

# Manual registration (includes cleanup)
./mcp-manager register --config mcp-config.yaml --verbose

# Check for errors
./mcp-manager serve --config mcp-config.yaml 2>&1 | grep -i error
```

### Server Not Responding
```bash
# Check gateway logs
./mcp-manager serve --config mcp-config.yaml

# Check if container is running
docker ps --filter label=managed-by=mcp-manager

# Check container logs
docker logs mcp-manager-git  # or other server name
```

### Auto-Sync Not Working
```bash
# Verify mcp-manager binary is up to date
make build

# Check auto-registration output when starting serve
./mcp-manager serve --config mcp-config.yaml 2>&1 | grep -i register

# Manually trigger registration with cleanup
./mcp-manager register --config mcp-config.yaml --verbose
```

## Notes

- Gateway starts instantly (~100ms)
- Auto-registration adds ~50-100ms to startup
- Auto-cleanup adds negligible overhead (~10ms)
- Containers spawn on first request (~2-5 seconds)
- Subsequent requests use existing containers (~50ms)
- Servers persist until gateway shutdown (no timeouts)
- No session persistence across gateway restarts (by design)
- Each server spawns only once per gateway session
- Mixed mode allows combining containerized and local servers
- Auto-sync ensures config always matches registration
- External servers (serena) preserved during cleanup
- Claude Code likely handles startup race conditions with retries