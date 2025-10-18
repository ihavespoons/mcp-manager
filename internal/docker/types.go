package docker

// ContainerConfig contains the configuration for creating a container
type ContainerConfig struct {
	Name          string   // Container name
	Image         string   // Docker image to use
	Command       []string // Command to run
	Env           []string // Environment variables in KEY=VALUE format
	Volumes       []string // Volume mounts in host:container format
	PortMappings  []string // Port mappings in host:container format (e.g., "8080:80" or "0:80" for random host port)
	Network       string   // Docker network to connect to
	RestartPolicy string   // Restart policy: "" (none), "unless-stopped", "always", "on-failure"
	OpenStdin     bool     // Keep stdin open for attach
	StdinOnce     bool     // Close stdin after one attach
	Tty           bool     // Allocate a pseudo-TTY
}

// ContainerStatus represents the status of a Docker container
type ContainerStatus struct {
	ID      string // Container ID
	Name    string // Container name
	State   string // Container state (running, exited, etc.)
	Running bool   // Whether the container is running
	Exists  bool   // Whether the container exists
}

// LogOptions contains options for retrieving container logs
type LogOptions struct {
	Follow bool // Follow log output
	Tail   int  // Number of lines to show from the end
}
