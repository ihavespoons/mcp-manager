# MCP Manager - Project Overview

## Purpose
`mcp-manager` is a CLI tool for managing Model Context Protocol (MCP) servers in Docker containers. It simplifies deployment and management by:
- Running MCP servers in isolated Docker containers
- Managing server lifecycle (start, stop, restart)
- Keeping MCP server images up to date
- Registering servers with AI agents (starting with Claude Code)
- Monitoring server health and status

## Tech Stack
- **Language**: Go 1.25.2
- **CLI Framework**: Cobra (github.com/spf13/cobra)
- **Configuration**: YAML (gopkg.in/yaml.v3)
- **Containerization**: Docker
- **Build Tool**: Make, Mise

## Project Structure
```
mcp-manager/
├── cmd/
│   └── mcp-manager/        # Main CLI entrypoint
│       └── main.go         # Cobra commands and CLI setup
├── internal/               # Private application packages
│   ├── config/            # Configuration loading and parsing
│   │   ├── config.go      # Configuration structs
│   │   └── loader.go      # Config file loading logic
│   ├── docker/            # Docker container management (planned)
│   └── server/            # MCP server lifecycle management (planned)
├── examples/
│   └── mcp-config.yaml    # Example configuration file
├── go.mod                 # Go module definition
├── go.sum                 # Go dependencies checksums
├── Makefile               # Build and development tasks
├── mise.toml              # Tool version management
└── README.md              # Project documentation
```

## Key Components
1. **CLI** (`cmd/mcp-manager`): User-facing command-line interface with commands for start, stop, status, list, logs, update, and validate
2. **Config** (`internal/config`): Configuration loading, parsing, validation, and default value setting
3. **Docker** (`internal/docker`): Docker container management (planned)
4. **Server** (`internal/server`): MCP server lifecycle management (planned)

## Configuration
- Configuration files use YAML format
- Primary config file: `mcp-config.yaml`
- Supports environment variable expansion (e.g., `${PWD}`)
- Default search locations:
  1. `./mcp-config.yaml`
  2. `./.mcp-manager/config.yaml`
  3. `~/.mcp-manager/config.yaml`

## Development Status
- Project is in early development
- Basic CLI structure and configuration parsing implemented
- Docker container management, Claude Code integration, and health monitoring are planned features (see README roadmap)
