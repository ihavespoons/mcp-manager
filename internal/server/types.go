package server

// ServerStatus represents the runtime status of an MCP server
type ServerStatus struct {
	Name        string // Server name
	Description string // Server description
	Image       string // Docker image
	State       string // Container state (running, exited, etc.)
	Running     bool   // Whether the server is running
	Exists      bool   // Whether the container exists
}

// ServerInfo represents basic information about a configured server
type ServerInfo struct {
	Name        string // Server name
	Description string // Server description
	Enabled     bool   // Whether the server is enabled
	Image       string // Docker image
}
