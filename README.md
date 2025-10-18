# MCP Manager

A CLI tool for managing Model Context Protocol (MCP) servers with Docker container support and an HTTP-to-stdio gateway.

## Overview

`mcp-manager` simplifies the deployment and management of MCP servers by:

- Running MCP servers as processes or in isolated Docker containers
- Providing an HTTP gateway for stdio-based MCP servers
- Managing server lifecycle (start, stop, restart)
- Keeping MCP server images up to date
- Registering servers with AI agents (Claude Code)
- Monitoring server health and status

### The stdio Server Problem

Many MCP servers are designed to communicate via stdio (standard input/output) and exit immediately when no input is provided. This creates a fundamental incompatibility with persistent Docker containers:

- **Problem**: Stdio MCP servers exit → restart policy triggers → infinite crash loop
- **Solution**: mcp-manager provides an HTTP gateway that spawns servers on-demand and manages their lifecycle

The gateway acts as a bridge, accepting HTTP requests and spawning ephemeral MCP server processes/containers as needed.

## Features

- **Configuration-driven**: Define all your MCP servers in a single YAML file
- **Multiple deployment modes**:
  - Direct container management for persistent services
  - HTTP gateway for stdio-based servers with full process and container support
- **Claude Code integration**: Register/unregister MCP servers with Claude Code CLI
- **Session management**: On-demand server spawning with automatic cleanup
- **Container lifecycle management**:
  - Graceful shutdown with proper container cleanup
  - Signal handling (SIGTERM, SIGINT) for clean shutdown
  - Startup cleanup of stale containers from previous runs
  - All containers labeled for easy identification
- **Protocol translation**: Full JSON-RPC 2.0 support with proper message framing
- **Docker integration**: Container isolation with attach API for secure server spawning
- **Environment variable support**: Pass configuration via environment variables
- **Easy updates**: Update all server images with a single command

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap ihavespoons/mcp-manager
brew install mcp-manager
```

### Download Binaries

Download pre-built binaries from the [releases page](https://github.com/ihavespoons/mcp-manager/releases):

- **macOS**: `mcp-manager-vX.Y.Z-darwin-amd64.tar.gz` (Intel) or `mcp-manager-vX.Y.Z-darwin-arm64.tar.gz` (Apple Silicon)
- **Linux**: `mcp-manager-vX.Y.Z-linux-amd64.tar.gz` (x86_64) or `mcp-manager-vX.Y.Z-linux-arm64.tar.gz` (ARM64)
- **Windows**: `mcp-manager-vX.Y.Z-windows-amd64.zip`

Extract and move to your PATH:
```bash
tar -xzf mcp-manager-*.tar.gz
sudo mv mcp-manager /usr/local/bin/
```

All releases include SHA256 checksums for verification.

### From Source

```bash
git clone https://github.com/ihavespoons/mcp-manager.git
cd mcp-manager
make build
# or
go build -o mcp-manager ./cmd/mcp-manager
```

### Prerequisites

- Go 1.25.2 or later
- Docker (for container-based servers)

## Quick Start

1. Create a configuration file (`mcp-config.yaml`):

```yaml
version: "1.0"

settings:
  data_dir: ~/.mcp-manager
  network: mcp-network
  log_level: info

servers:
  - name: filesystem
    enabled: true
    description: "Provides filesystem access capabilities"
    docker:
      image: mcp/filesystem:latest
      pull_policy: if-not-present
    command: ["mcp-server-filesystem"]
    args: ["--root", "/workspace"]
    volumes:
      - host_path: ${PWD}
        container_path: /workspace
        read_only: false
    agents:
      - claude-code
```

2. Start your MCP servers:

```bash
mcp-manager start
```

3. Check server status:

```bash
mcp-manager status
```

## Usage

### Commands

#### Container Management
- `mcp-manager start [server-name]` - Start one or all MCP servers
- `mcp-manager stop [server-name]` - Stop one or all MCP servers
- `mcp-manager status [server-name]` - Show status of MCP servers
- `mcp-manager list` - List all configured MCP servers
- `mcp-manager logs <server-name>` - Show logs from an MCP server
- `mcp-manager update [server-name]` - Update MCP server Docker images
- `mcp-manager validate` - Validate configuration file

#### Gateway
- `mcp-manager gateway` - Start the HTTP-to-stdio gateway server
  - `--host` - Gateway host address (default: 0.0.0.0)
  - `--port` - Gateway port (default: 8080)
  - `--mode` - Server mode: container or process (default: process)
  - `--timeout` - Session timeout (default: 30m)

#### Claude Code Integration
- `mcp-manager register` - Register mcp-manager and all enabled MCP servers with Claude Code
- `mcp-manager unregister` - Unregister mcp-manager and all configured servers from Claude Code
- `mcp-manager serve` - Run mcp-manager as an MCP server (stdio) with integrated gateway

### Global Flags

- `-c, --config` - Specify config file location (default: searches standard locations)
- `-v, --verbose` - Enable verbose output

### Examples

#### Container Management
Start all servers:
```bash
mcp-manager start --all
```

Start a specific server:
```bash
mcp-manager start filesystem
```

Follow logs from a server:
```bash
mcp-manager logs -f filesystem
```

Update all server images:
```bash
mcp-manager update --all
```

#### Gateway Usage

Start the gateway in process mode (recommended, fully tested):
```bash
mcp-manager gateway --mode process --port 52080
```

Start the gateway in container mode (for Docker isolation):
```bash
mcp-manager gateway --mode container --port 52080
```

Test gateway health:
```bash
curl http://localhost:52080/health
```

List available servers:
```bash
curl http://localhost:52080/servers
```

Send MCP request:
```bash
curl -X POST http://localhost:52080/mcp \
  -H "X-MCP-Server: filesystem" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"client","version":"1.0.0"}}}'
```

**Mode Comparison:**
- **Process Mode**: Spawns MCP servers as local processes. Fully tested with npm packages. Recommended for development.
- **Container Mode**: Spawns MCP servers as Docker containers. Provides isolation and resource limits. Requires MCP server Docker images.

#### Claude Code Integration

Register mcp-manager and all enabled servers with Claude Code:
```bash
mcp-manager register --config mcp-config.yaml
```

This will:
1. Register `mcp-manager` itself as a stdio MCP server that runs `mcp-manager serve`
2. Register each enabled server as an HTTP endpoint through the gateway (e.g., `http://localhost:52080/mcp/filesystem`)
3. Update `~/.claude.json` for the current project

Verify registration:
```bash
cat ~/.claude.json | jq '.projects["/path/to/your/project"].mcpServers'
```

Unregister everything:
```bash
mcp-manager unregister
```

**How it works:**
- `mcp-manager serve` starts the gateway and exposes management tools (`list_servers`, `server_status`, `gateway_status`)
- Individual servers (filesystem, git, etc.) are accessed via HTTP endpoints through the gateway
- Each server maintains its own namespace and can be referenced by name in Claude Code

## Configuration

The configuration file defines all MCP servers and their settings. See `examples/mcp-config.yaml` for a complete example.

### Configuration Locations

`mcp-manager` searches for configuration files in the following order:

1. Path specified with `-c` flag
2. `./mcp-config.yaml`
3. `./.mcp-manager/config.yaml`
4. `~/.mcp-manager/config.yaml`

### Configuration Structure

```yaml
version: "1.0"

settings:
  data_dir: ~/.mcp-manager       # Runtime data directory
  network: mcp-network            # Docker network name
  log_level: info                 # Log level: debug, info, warn, error

servers:
  - name: server-name             # Unique server identifier
    enabled: true                 # Whether this server should be managed
    description: "Description"    # Human-readable description

    docker:
      image: org/image:tag        # Docker image to use
      pull_policy: if-not-present # always, never, if-not-present

    http_path: /mcp               # Optional: HTTP path for containers with HTTP endpoints
                                   # Appended to registration URL for Claude Code
                                   # Example: http://localhost:52080/mcp/server-name/mcp

    command: ["executable"]       # Command to run
    args: ["--flag", "value"]     # Command arguments

    env:                          # Environment variables
      - name: VAR_NAME
        value: "value"
      - name: VAR_FROM_ENV
        value_from_env: HOST_VAR  # Pass through from host

    volumes:                      # Volume mounts
      - host_path: /host/path
        container_path: /container/path
        read_only: false

    health_check:                 # Health check configuration
      enabled: true
      interval: 30s
      timeout: 10s
      retries: 3

    agents:                       # Agents to register with
      - claude-code
```

## Architecture

### Components

- **CLI** (`cmd/mcp-manager`): User-facing command-line interface
- **Config** (`internal/config`): Configuration loading and parsing
- **Docker** (`internal/docker`): Docker container management
- **Server** (`internal/server`): MCP server lifecycle management
- **Gateway** (`internal/gateway`): HTTP-to-stdio protocol gateway
  - Session management with timeout cleanup
  - Process and container spawning
  - JSON-RPC 2.0 message handling
- **Claude Code** (`internal/claudecode`): Claude Code configuration management

### Gateway Architecture

```
HTTP Client
    ↓
Gateway (persistent)
    ↓
Server Manager
    ↓
Persistent Server Instances (one per server name)
    ↓
stdio communication (JSON-RPC)
    ↓
Tool execution
```

**Container Lifecycle:**

The gateway maintains persistent server instances (one per server name) for the duration of its runtime:

1. **Startup**: Cleans up any stale containers from previous runs
2. **First Request**: Spawns container for the requested server (e.g., "filesystem")
3. **Subsequent Requests**: Reuses existing container
4. **Shutdown**: Properly cleans up all containers on exit

**Cleanup Scenarios:**
- **Normal shutdown**: stdin close from Claude Code triggers cleanup
- **Signal shutdown**: SIGTERM/SIGINT caught and handled gracefully
- **Error shutdown**: Context cancellation triggers cleanup
- **Next startup**: Safety net cleanup removes any missed containers

All containers are labeled with `managed-by=mcp-manager` for easy identification.

### Deployment Flows

#### Direct Container Mode
1. Load configuration file
2. Create Docker network if needed
3. For each enabled server:
   - Pull image (if needed)
   - Start container with specified configuration
   - Monitor container status

#### Gateway Mode
1. Start HTTP gateway server
2. On client request:
   - Create session with unique ID
   - Spawn MCP server (process or container)
   - Forward JSON-RPC messages via stdio
   - Manage session lifecycle (30min timeout default)

#### Claude Code Integration
1. Load mcp-manager configuration
2. Register mcp-manager as stdio server that runs `mcp-manager serve`
3. Register each enabled server as HTTP endpoint through gateway
4. Update `~/.claude.json` with per-project server registrations
5. Servers become available in Claude Code after restart

**Registration Details:**
- `mcp-manager` → stdio server (provides management tools)
- Each MCP server → HTTP endpoint at `http://localhost:{gateway_port}/mcp/{server-name}`
- Gateway port defaults to 52080 (configurable in settings.gateway_port)

## Roadmap

### Completed ✅
- [x] Basic project structure
- [x] Configuration file format
- [x] CLI command structure
- [x] Docker container management
- [x] HTTP-to-stdio gateway (process mode)
- [x] HTTP-to-stdio gateway (container mode with Docker attach)
- [x] JSON-RPC 2.0 protocol handling
- [x] Server instance management (persistent per server)
- [x] Container lifecycle management with graceful shutdown
- [x] Signal handling (SIGTERM, SIGINT) for cleanup
- [x] Startup cleanup of stale containers
- [x] Claude Code integration (register/unregister)
- [x] Environment variable handling
- [x] Volume mount configuration
- [x] Comprehensive testing suite

### In Progress 🚧
- [ ] Production logging and metrics

### Planned 📋
- [ ] Health monitoring for persistent containers
- [ ] SSE streaming support for gateway
- [ ] Connection pooling and optimization
- [ ] Support for additional agents (Cursor, etc.)
- [ ] Web UI for server management
- [ ] Automatic updates with version checking
- [ ] Server state persistence

## Documentation

Comprehensive documentation is available in the `docs/` directory:

### Release and Distribution
- **[Release Process](docs/RELEASE.md)** - Complete guide to releasing new versions with GitHub Actions CI/CD and automated Homebrew tap updates
- **[Homebrew Tap Setup](docs/HOMEBREW_TAP_SETUP.md)** - Detailed instructions for setting up and maintaining the Homebrew tap repository

### Implementation
- **[Implementation Summary](docs/IMPLEMENTATION_SUMMARY.md)** - Complete implementation history across all development phases, including:
  - Gateway implementation for HTTP-to-stdio bridging
  - Claude Code integration and registration
  - Docker container mode with attach API
  - Container lifecycle management and cleanup
  - Architecture diagrams and technical details

### Testing
- **[Test Results](docs/testing/TEST_RESULTS.md)** - Comprehensive test suite results including unit tests, integration tests, and manual testing
- **[Gateway Test Results](docs/testing/GATEWAY_TEST_RESULTS.md)** - Detailed test results for process mode gateway functionality
- **[Container Mode Test Results](docs/testing/CONTAINER_MODE_TEST_RESULTS.md)** - Test results for Docker container mode with stream demultiplexing

### Project Configuration
- **[CLAUDE.md](CLAUDE.md)** - Instructions for working with Claude Code and Serena MCP for enhanced development assistance
- **[Serena Tools Guide](SERENA_TOOLS_GUIDE.md)** - Comprehensive reference for using Serena MCP semantic code tools effectively

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Functional Source License, Version 1.1, MIT Future License (FSL-1.1-MIT)

See [LICENSE](LICENSE) file for details.

**Summary**: This is a source-available license that converts to MIT after two years. You can use, modify, and redistribute the software for non-production use. For production use beyond the terms of this license, please contact the licensor.

## Acknowledgments

Built to work with the [Model Context Protocol](https://modelcontextprotocol.io/) and [Claude Code](https://docs.anthropic.com/claude/docs/claude-code).
