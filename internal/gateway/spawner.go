package gateway

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
)

// spawnContainer spawns an MCP server in a Docker container (no restart policy)
func (s *Session) spawnContainer(ctx context.Context, gateway *Gateway, serverConfig config.Server) error {
	// Build container config
	containerConfig, err := buildContainerConfig(serverConfig, gateway.config.Settings.Network)
	if err != nil {
		return fmt.Errorf("failed to build container config: %w", err)
	}

	// Override the name to include session ID to avoid conflicts
	containerConfig.Name = fmt.Sprintf("%s-%s", serverConfig.Name, s.ID[:8])

	// Pull image if needed
	if err := gateway.docker.PullImage(ctx, serverConfig.Docker.Image, serverConfig.Docker.PullPolicy); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container (no restart policy - ephemeral)
	containerID, err := gateway.docker.CreateContainer(ctx, containerConfig)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	s.ContainerID = containerID

	// Attach to container to get stdin/stdout pipes
	stdinPipe, stdoutPipe, err := attachToContainer(ctx, gateway.docker, containerID)
	if err != nil {
		// Cleanup container on failure
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to attach to container: %w", err)
	}

	s.stdin = stdinPipe
	s.stdout = stdoutPipe

	// Initialize message reader/writer for JSON-RPC protocol
	s.messageWriter = NewMessageWriter(stdinPipe)
	s.messageReader = NewMessageReader(stdoutPipe)

	// Start the container
	if err := gateway.docker.StartContainer(ctx, containerID); err != nil {
		// Cleanup on failure
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// spawnProcess spawns an MCP server as a local process
func (s *Session) spawnProcess(ctx context.Context, gateway *Gateway, serverConfig config.Server) error {
	// Build command
	args := append(serverConfig.Command, serverConfig.Args...)
	if len(args) == 0 {
		return fmt.Errorf("no command specified for server %s", serverConfig.Name)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Set environment variables
	// Start with parent environment, then add/override with config
	cmd.Env = os.Environ()
	configEnv, err := buildEnv(serverConfig.Env)
	if err != nil {
		return fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = append(cmd.Env, configEnv...)

	// Redirect stderr to discard diagnostic messages (they're not part of JSON-RPC protocol)
	cmd.Stderr = os.Stderr

	// Get stdin/stdout pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	s.stdin = stdin
	s.stdout = stdout

	// Initialize message reader/writer for JSON-RPC protocol
	s.messageWriter = NewMessageWriter(stdin)
	s.messageReader = NewMessageReader(stdout)

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	s.ProcessID = cmd.Process.Pid

	// Handle process cleanup when it exits
	go func() {
		cmd.Wait()
	}()

	return nil
}

// attachToContainer attaches to a container and returns stdin/stdout pipes
func attachToContainer(ctx context.Context, dockerClient *docker.Client, containerID string) (io.WriteCloser, io.ReadCloser, error) {
	stdin, stdout, err := dockerClient.AttachContainer(ctx, containerID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to attach to container: %w", err)
	}
	return stdin, stdout, nil
}

// buildContainerConfig builds a Docker container config for the session
func buildContainerConfig(serverConfig config.Server, network string) (*docker.ContainerConfig, error) {
	// Build command
	command := append(serverConfig.Command, serverConfig.Args...)

	// Build environment variables
	env, err := buildEnv(serverConfig.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}

	// Build volumes
	volumes, err := buildVolumes(serverConfig.Volumes)
	if err != nil {
		return nil, fmt.Errorf("failed to build volumes: %w", err)
	}

	return &docker.ContainerConfig{
		Name:          serverConfig.Name,
		Image:         serverConfig.Docker.Image,
		Command:       command,
		Env:           env,
		Volumes:       volumes,
		Network:       network,
		RestartPolicy: "",     // No restart policy for ephemeral gateway containers
		OpenStdin:     true,   // Keep stdin open for attach
		StdinOnce:     false,  // Don't close stdin after first attach
		Tty:           false,  // No TTY needed for JSON-RPC
	}, nil
}

// buildEnv converts config.EnvVar to environment strings
func buildEnv(envVars []config.EnvVar) ([]string, error) {
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
func buildVolumes(volumes []config.Volume) ([]string, error) {
	var binds []string
	for _, v := range volumes {
		hostPath := v.HostPath

		// Expand tilde to home directory
		if strings.HasPrefix(hostPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			hostPath = strings.Replace(hostPath, "~", homeDir, 1)
		}

		// Expand environment variables in paths
		hostPath = os.ExpandEnv(hostPath)

		// Build bind mount string
		bind := fmt.Sprintf("%s:%s", hostPath, v.ContainerPath)
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}
	return binds, nil
}
