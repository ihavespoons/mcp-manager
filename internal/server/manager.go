package server

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
)

// Manager manages the lifecycle of MCP servers
type Manager struct {
	docker *docker.Client
	config *config.Config
}

// NewManager creates a new server manager
func NewManager(dockerClient *docker.Client, cfg *config.Config) *Manager {
	return &Manager{
		docker: dockerClient,
		config: cfg,
	}
}

// Start starts one or all MCP servers
func (m *Manager) Start(ctx context.Context, serverName string) error {
	servers := m.getServersToOperate(serverName)
	if len(servers) == 0 {
		if serverName == "" {
			return fmt.Errorf("no enabled servers found")
		}
		return fmt.Errorf("server %s not found or not enabled", serverName)
	}

	// Ensure network exists
	if err := m.docker.CreateNetwork(ctx, m.config.Settings.Network); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Start each server
	for _, server := range servers {
		if err := m.startServer(ctx, server); err != nil {
			return fmt.Errorf("failed to start server %s: %w", server.Name, err)
		}
	}

	return nil
}

// Stop stops one or all MCP servers
func (m *Manager) Stop(ctx context.Context, serverName string) error {
	servers := m.getServersToOperate(serverName)
	if len(servers) == 0 {
		if serverName == "" {
			return fmt.Errorf("no enabled servers found")
		}
		return fmt.Errorf("server %s not found", serverName)
	}

	// Stop each server
	for _, server := range servers {
		if err := m.stopServer(ctx, server); err != nil {
			return fmt.Errorf("failed to stop server %s: %w", server.Name, err)
		}
	}

	return nil
}

// Status returns the status of one or all MCP servers
func (m *Manager) Status(ctx context.Context, serverName string) ([]ServerStatus, error) {
	servers := m.getServersToOperate(serverName)
	if len(servers) == 0 {
		if serverName == "" {
			return nil, fmt.Errorf("no enabled servers found")
		}
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	var statuses []ServerStatus
	for _, server := range servers {
		status, err := m.getServerStatus(ctx, server)
		if err != nil {
			return nil, fmt.Errorf("failed to get status for server %s: %w", server.Name, err)
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// List returns all configured servers
func (m *Manager) List() []ServerInfo {
	var servers []ServerInfo
	for _, server := range m.config.Servers {
		servers = append(servers, ServerInfo{
			Name:        server.Name,
			Description: server.Description,
			Enabled:     server.Enabled,
			Image:       server.Docker.Image,
		})
	}
	return servers
}

// GetLogs returns logs from a server
func (m *Manager) GetLogs(ctx context.Context, serverName string, follow bool, tail int) (*docker.LogOptions, error) {
	server := m.findServer(serverName)
	if server == nil {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	return &docker.LogOptions{
		Follow: follow,
		Tail:   tail,
	}, nil
}

// startServer starts a single MCP server
func (m *Manager) startServer(ctx context.Context, server config.Server) error {
	// Check if container already exists
	status, err := m.docker.GetContainerStatus(ctx, server.Name)
	if err != nil {
		return err
	}

	if status.Exists {
		if status.Running {
			return fmt.Errorf("server %s is already running", server.Name)
		}
		// Container exists but not running, just start it
		return m.docker.StartContainer(ctx, status.ID)
	}

	// Pull image if needed
	if err := m.docker.PullImage(ctx, server.Docker.Image, server.Docker.PullPolicy); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container config
	containerConfig, err := m.buildContainerConfig(server)
	if err != nil {
		return fmt.Errorf("failed to build container config: %w", err)
	}

	// Create container
	containerID, err := m.docker.CreateContainer(ctx, containerConfig)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := m.docker.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// stopServer stops a single MCP server
func (m *Manager) stopServer(ctx context.Context, server config.Server) error {
	status, err := m.docker.GetContainerStatus(ctx, server.Name)
	if err != nil {
		return err
	}

	if !status.Exists {
		return fmt.Errorf("server %s is not running", server.Name)
	}

	if !status.Running {
		return nil // Already stopped
	}

	// Stop with timeout
	timeout := 10 // seconds
	return m.docker.StopContainer(ctx, status.ID, timeout)
}

// getServerStatus gets the status of a single server
func (m *Manager) getServerStatus(ctx context.Context, server config.Server) (ServerStatus, error) {
	status, err := m.docker.GetContainerStatus(ctx, server.Name)
	if err != nil {
		return ServerStatus{}, err
	}

	return ServerStatus{
		Name:        server.Name,
		Description: server.Description,
		Image:       server.Docker.Image,
		State:       status.State,
		Running:     status.Running,
		Exists:      status.Exists,
	}, nil
}

// buildContainerConfig converts a config.Server to docker.ContainerConfig
func (m *Manager) buildContainerConfig(server config.Server) (*docker.ContainerConfig, error) {
	// Build command
	command := append(server.Command, server.Args...)

	// Build environment variables
	env, err := m.buildEnv(server.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}

	// Build volumes
	volumes, err := m.buildVolumes(server.Volumes)
	if err != nil {
		return nil, fmt.Errorf("failed to build volumes: %w", err)
	}

	return &docker.ContainerConfig{
		Name:    server.Name,
		Image:   server.Docker.Image,
		Command: command,
		Env:     env,
		Volumes: volumes,
		Network: m.config.Settings.Network,
	}, nil
}

// buildEnv converts config.EnvVar to Docker environment format
func (m *Manager) buildEnv(envVars []config.EnvVar) ([]string, error) {
	var env []string
	for _, e := range envVars {
		var value string
		if e.ValueFromEnv != "" {
			// Get value from host environment
			value = os.Getenv(e.ValueFromEnv)
			if value == "" {
				return nil, fmt.Errorf("environment variable %s not found in host environment", e.ValueFromEnv)
			}
		} else {
			value = e.Value
		}
		env = append(env, fmt.Sprintf("%s=%s", e.Name, value))
	}
	return env, nil
}

// buildVolumes converts config.Volume to Docker bind mount format
func (m *Manager) buildVolumes(volumes []config.Volume) ([]string, error) {
	var binds []string
	for _, v := range volumes {
		// Expand environment variables in paths
		hostPath := os.ExpandEnv(v.HostPath)

		// Build bind mount string
		bind := fmt.Sprintf("%s:%s", hostPath, v.ContainerPath)
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}
	return binds, nil
}

// getServersToOperate returns the servers to operate on
func (m *Manager) getServersToOperate(serverName string) []config.Server {
	if serverName == "" {
		// Return all enabled servers
		return m.config.GetEnabledServers()
	}

	// Return specific server if found and enabled
	server := m.findServer(serverName)
	if server != nil && server.Enabled {
		return []config.Server{*server}
	}

	return nil
}

// findServer finds a server by name
func (m *Manager) findServer(name string) *config.Server {
	for _, server := range m.config.Servers {
		if strings.EqualFold(server.Name, name) {
			return &server
		}
	}
	return nil
}
