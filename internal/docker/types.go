package docker

// ContainerConfig contains the configuration for creating a container
type ContainerConfig struct {
	Name    string   // Container name
	Image   string   // Docker image to use
	Command []string // Command to run
	Env     []string // Environment variables in KEY=VALUE format
	Volumes []string // Volume mounts in host:container format
	Network string   // Docker network to connect to
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
