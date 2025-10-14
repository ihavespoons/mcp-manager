# MCP Manager

A CLI tool for managing Model Context Protocol (MCP) servers in Docker containers.

## Overview

`mcp-manager` simplifies the deployment and management of MCP servers by:

- Running MCP servers in isolated Docker containers
- Managing server lifecycle (start, stop, restart)
- Keeping MCP server images up to date
- Registering servers with AI agents (starting with Claude Code)
- Monitoring server health and status

## Features

- **Configuration-driven**: Define all your MCP servers in a single YAML file
- **Docker-based**: Each MCP server runs in its own container for isolation and portability
- **Agent integration**: Automatically register MCP servers with supported AI agents
- **Health monitoring**: Built-in health checks to ensure servers are running correctly
- **Easy updates**: Update all server images with a single command
- **Environment variable support**: Pass configuration via environment variables

## Installation

### From Source

```bash
git clone https://github.com/bengittins/mcp-manager.git
cd mcp-manager
go build -o mcp-manager ./cmd/mcp-manager
```

### Prerequisites

- Go 1.21 or later
- Docker

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

- `mcp-manager start [server-name]` - Start one or all MCP servers
- `mcp-manager stop [server-name]` - Stop one or all MCP servers
- `mcp-manager status [server-name]` - Show status of MCP servers
- `mcp-manager list` - List all configured MCP servers
- `mcp-manager logs <server-name>` - Show logs from an MCP server
- `mcp-manager update [server-name]` - Update MCP server Docker images
- `mcp-manager validate` - Validate configuration file

### Global Flags

- `-c, --config` - Specify config file location (default: searches standard locations)
- `-v, --verbose` - Enable verbose output

### Examples

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

- **CLI**: User-facing command-line interface (cmd/mcp-manager)
- **Config**: Configuration loading and parsing (internal/config)
- **Docker**: Docker container management (internal/docker)
- **Server**: MCP server lifecycle management (internal/server)

### Flow

1. Load configuration file
2. Create Docker network if needed
3. For each enabled server:
   - Pull image (if needed)
   - Start container with specified configuration
   - Register with configured agents
   - Start health monitoring

## Roadmap

- [x] Basic project structure
- [x] Configuration file format
- [x] CLI command structure
- [ ] Docker container management
- [ ] Claude Code integration
- [ ] Health monitoring
- [ ] Server registry and state management
- [ ] Automatic updates
- [ ] Support for additional agents (Cursor, etc.)
- [ ] Web UI for server management

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Acknowledgments

Built to work with the [Model Context Protocol](https://modelcontextprotocol.io/) and [Claude Code](https://docs.anthropic.com/claude/docs/claude-code).
