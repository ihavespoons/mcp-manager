# Suggested Commands

## Build Commands
- **Build binary**: `make build`
  - Builds the `mcp-manager` binary in the current directory
  - Includes version, commit, and build date in the binary
- **Install to GOPATH**: `make install`
  - Installs the binary to `$GOPATH/bin`
- **Build and run**: `make run ARGS="command"`
  - Example: `make run ARGS="start --all"`
  - Example: `make run ARGS="status"`

## Testing Commands
- **Run tests**: `make test`
  - Runs all tests with race detection
  - Generates coverage report in `coverage.out`
- **View coverage**: `make coverage`
  - Runs tests and opens coverage report in browser

## Code Quality Commands
- **Format code**: `make fmt`
  - Formats all Go code using `go fmt`
- **Run go vet**: `make vet`
  - Runs `go vet` to find suspicious code
- **Run linter**: `make lint`
  - Requires `golangci-lint` to be installed
  - Runs comprehensive linting checks

## Dependency Management
- **Tidy modules**: `make tidy`
  - Cleans up go.mod and go.sum
- **Download dependencies**: `make deps`
  - Downloads all module dependencies

## Cleanup
- **Clean build artifacts**: `make clean`
  - Removes binary, coverage files, and dist directory

## Application Commands
Once built, the `mcp-manager` binary supports:
- `./mcp-manager start [server-name]` - Start MCP servers
- `./mcp-manager stop [server-name]` - Stop MCP servers
- `./mcp-manager status [server-name]` - Show server status
- `./mcp-manager list` - List configured servers
- `./mcp-manager logs <server-name>` - Show server logs
- `./mcp-manager update [server-name]` - Update server images
- `./mcp-manager validate` - Validate configuration file

Global flags:
- `-c, --config <path>` - Specify config file location
- `-v, --verbose` - Enable verbose output

## Darwin (macOS) System Commands
Standard Unix commands work on Darwin:
- `git` - Version control
- `ls` - List directory contents
- `cd` - Change directory
- `grep` - Search text patterns
- `find` - Find files
- `cat` - Display file contents
- `docker` - Docker container management (required for mcp-manager)
