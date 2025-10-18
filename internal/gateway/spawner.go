package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
	"github.com/bengittins/mcp-manager/internal/log"
)

// spawnContainer spawns an MCP server in a Docker container (persistent, one per server)
func (s *ServerInstance) spawnContainer(ctx context.Context, gateway *Gateway, serverConfig config.Server) error {
	log.Info("Spawning container for server", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
	})

	// Build container config
	containerConfig, err := buildContainerConfig(serverConfig, gateway.config.Settings.Network)
	if err != nil {
		log.Error("Failed to build container config", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to build container config: %w", err)
	}

	// Use server name directly (one container per server)
	containerConfig.Name = serverConfig.Name

	// Remove any existing container with the same name (from previous failed attempts)
	log.Debug("Removing any existing container", map[string]interface{}{
		"server_name":    serverConfig.Name,
		"container_name": serverConfig.Name,
	})
	_ = gateway.docker.RemoveContainer(ctx, serverConfig.Name, true)

	// Pull image if needed
	log.Info("Pulling Docker image", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
		"pull_policy": serverConfig.Docker.PullPolicy,
	})
	if err := gateway.docker.PullImage(ctx, serverConfig.Docker.Image, serverConfig.Docker.PullPolicy); err != nil {
		log.Error("Failed to pull image", map[string]interface{}{
			"server_name": serverConfig.Name,
			"image":       serverConfig.Docker.Image,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container (no restart policy - ephemeral)
	log.Debug("Creating container", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
	})
	containerID, err := gateway.docker.CreateContainer(ctx, containerConfig)
	if err != nil {
		log.Error("Failed to create container", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to create container: %w", err)
	}

	s.ContainerID = containerID
	log.Info("Container created", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	// Start the container FIRST
	log.Debug("Starting container", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})
	if err := gateway.docker.StartContainer(ctx, containerID); err != nil {
		log.Error("Failed to start container", map[string]interface{}{
			"server_name":  serverConfig.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		// Cleanup on failure
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to start container: %w", err)
	}
	log.Info("Container started", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	// THEN attach to get stdin/stdout pipes
	log.Debug("Attaching to container", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})
	stdinPipe, stdoutPipe, err := attachToContainer(ctx, gateway.docker, containerID)
	if err != nil {
		log.Error("Failed to attach to container", map[string]interface{}{
			"server_name":  serverConfig.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		// Cleanup container on failure
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to attach to container: %w", err)
	}
	log.Info("Successfully attached to container", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	s.stdin = stdinPipe
	s.stdout = stdoutPipe

	// Initialize message reader/writer for JSON-RPC protocol
	s.messageWriter = NewMessageWriter(stdinPipe)
	s.messageReader = NewMessageReader(stdoutPipe)

	log.Info("Container spawn complete", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	return nil
}

// spawnProcess spawns an MCP server as a local process (persistent, one per server)
func (s *ServerInstance) spawnProcess(ctx context.Context, gateway *Gateway, serverConfig config.Server) error {
	log.Info("Spawning process for server", map[string]interface{}{
		"server_name": serverConfig.Name,
	})

	// Build command
	args := append(serverConfig.Command, serverConfig.Args...)
	if len(args) == 0 {
		// If no command specified, try to use npx with the server name as a fallback
		// This allows Docker images with default commands to work in process mode
		args = []string{"npx", "-y", fmt.Sprintf("@modelcontextprotocol/server-%s", serverConfig.Name)}
	}

	log.Debug("Building process command", map[string]interface{}{
		"server_name": serverConfig.Name,
		"command":     args,
	})

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Set environment variables
	// Start with parent environment, then add/override with config
	cmd.Env = os.Environ()
	configEnv, err := buildEnv(serverConfig.Env)
	if err != nil {
		log.Error("Failed to build environment", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = append(cmd.Env, configEnv...)

	// Redirect stderr to discard diagnostic messages (they're not part of JSON-RPC protocol)
	cmd.Stderr = os.Stderr

	// Get stdin/stdout pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("Failed to create stdin pipe", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Failed to create stdout pipe", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	s.stdin = stdin
	s.stdout = stdout

	// Initialize message reader/writer for JSON-RPC protocol
	s.messageWriter = NewMessageWriter(stdin)
	s.messageReader = NewMessageReader(stdout)

	// Start the process
	log.Debug("Starting process", map[string]interface{}{
		"server_name": serverConfig.Name,
		"command":     args[0],
	})
	if err := cmd.Start(); err != nil {
		log.Error("Failed to start process", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to start process: %w", err)
	}

	s.ProcessID = cmd.Process.Pid
	log.Info("Process started successfully", map[string]interface{}{
		"server_name": serverConfig.Name,
		"process_id":  s.ProcessID,
	})

	// Handle process cleanup when it exits
	go func() {
		cmd.Wait()
		log.Info("Process exited", map[string]interface{}{
			"server_name": serverConfig.Name,
			"process_id":  s.ProcessID,
		})
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
		RestartPolicy: "",    // No restart policy for ephemeral gateway containers
		OpenStdin:     true,  // Keep stdin open for attach
		StdinOnce:     false, // Don't close stdin after first attach
		Tty:           false, // No TTY needed for JSON-RPC
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

// spawnHTTPContainer spawns an MCP server in a Docker container that exposes HTTP
func (s *ServerInstance) spawnHTTPContainer(ctx context.Context, gateway *Gateway, serverConfig config.Server) error {
	log.Info("Spawning HTTP container for server", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
		"port":        serverConfig.Port,
	})

	// Build container config
	containerConfig, err := buildContainerConfig(serverConfig, gateway.config.Settings.Network)
	if err != nil {
		log.Error("Failed to build container config", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to build container config: %w", err)
	}

	// Use server name directly (one container per server)
	containerConfig.Name = serverConfig.Name

	// Add port mapping for HTTP server
	containerConfig.PortMappings = []string{fmt.Sprintf("0:%d", serverConfig.Port)}

	// Remove any existing container with the same name
	log.Debug("Removing any existing container", map[string]interface{}{
		"server_name":    serverConfig.Name,
		"container_name": serverConfig.Name,
	})
	_ = gateway.docker.RemoveContainer(ctx, serverConfig.Name, true)

	// Pull image if needed
	log.Info("Pulling Docker image", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
		"pull_policy": serverConfig.Docker.PullPolicy,
	})
	if err := gateway.docker.PullImage(ctx, serverConfig.Docker.Image, serverConfig.Docker.PullPolicy); err != nil {
		log.Error("Failed to pull image", map[string]interface{}{
			"server_name": serverConfig.Name,
			"image":       serverConfig.Docker.Image,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container
	log.Debug("Creating container", map[string]interface{}{
		"server_name": serverConfig.Name,
		"image":       serverConfig.Docker.Image,
	})
	containerID, err := gateway.docker.CreateContainer(ctx, containerConfig)
	if err != nil {
		log.Error("Failed to create container", map[string]interface{}{
			"server_name": serverConfig.Name,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to create container: %w", err)
	}

	s.ContainerID = containerID
	log.Info("Container created", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	// Start the container
	log.Debug("Starting container", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})
	if err := gateway.docker.StartContainer(ctx, containerID); err != nil {
		log.Error("Failed to start container", map[string]interface{}{
			"server_name":  serverConfig.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		// Cleanup on failure
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to start container: %w", err)
	}
	log.Info("Container started", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
	})

	// Get the mapped port
	mappedPort, err := gateway.docker.GetMappedPort(ctx, containerID, serverConfig.Port)
	if err != nil {
		log.Error("Failed to get mapped port", map[string]interface{}{
			"server_name":  serverConfig.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		gateway.docker.RemoveContainer(ctx, containerID, true)
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Build HTTP URL
	s.httpURL = fmt.Sprintf("http://localhost:%d", mappedPort)
	// Append http_path if configured (e.g., "/mcp" for context7)
	if serverConfig.HTTPPath != "" {
		s.httpURL = s.httpURL + serverConfig.HTTPPath
	}
	s.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Wait for HTTP server to be ready before returning
	log.Info("Waiting for HTTP server to become ready", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
		"http_url":     s.httpURL,
	})

	// Create a temporary HTTP client with shorter timeout for health checks
	healthClient := &http.Client{
		Timeout: 2 * time.Second,
	}

	maxAttempts := 30 // 30 seconds max wait time
	for i := 0; i < maxAttempts; i++ {
		// Try to make an HTTP GET request to verify server is responding
		// We don't care about the response code, just that the HTTP server is accepting requests
		resp, err := healthClient.Get(s.httpURL)
		if err == nil {
			resp.Body.Close()
			log.Info("HTTP server ready", map[string]interface{}{
				"server_name":   serverConfig.Name,
				"attempt":       i + 1,
				"status_code":   resp.StatusCode,
				"health_method": "http_get",
			})
			break
		}

		// Log the error for debugging but continue retrying
		log.Debug("HTTP health check failed, retrying", map[string]interface{}{
			"server_name": serverConfig.Name,
			"attempt":     i + 1,
			"error":       err.Error(),
		})

		if i == maxAttempts-1 {
			// Timeout - cleanup and fail
			log.Error("HTTP server did not become ready", map[string]interface{}{
				"server_name":  serverConfig.Name,
				"container_id": containerID,
				"timeout":      maxAttempts,
				"last_error":   err.Error(),
			})
			gateway.docker.RemoveContainer(ctx, containerID, true)
			return fmt.Errorf("HTTP server did not become ready after %d seconds: %w", maxAttempts, err)
		}

		// Wait before retry
		time.Sleep(1 * time.Second)
	}

	log.Info("HTTP container spawn complete", map[string]interface{}{
		"server_name":  serverConfig.Name,
		"container_id": containerID,
		"http_url":     s.httpURL,
		"http_path":    serverConfig.HTTPPath,
	})

	return nil
}

// connectHTTPExternal connects to an external HTTP-based MCP server
func (s *ServerInstance) connectHTTPExternal(ctx context.Context, serverConfig config.Server) error {
	log.Info("Connecting to external HTTP server", map[string]interface{}{
		"server_name": serverConfig.Name,
		"url":         serverConfig.URL,
	})

	s.httpURL = serverConfig.URL
	s.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	log.Info("HTTP external connection complete", map[string]interface{}{
		"server_name": serverConfig.Name,
		"http_url":    s.httpURL,
	})

	return nil
}
