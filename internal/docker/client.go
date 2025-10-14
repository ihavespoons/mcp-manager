package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client wraps the Docker client and provides high-level operations
type Client struct {
	cli *client.Client
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{cli: cli}, nil
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

// Ping checks if the Docker daemon is accessible
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping Docker daemon: %w", err)
	}
	return nil
}

// PullImage pulls a Docker image with the specified pull policy
func (c *Client) PullImage(ctx context.Context, imageName string, pullPolicy string) error {
	// Check if image exists locally
	exists, err := c.imageExists(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	// Apply pull policy
	switch pullPolicy {
	case "never":
		if !exists {
			return fmt.Errorf("image %s does not exist locally and pull policy is 'never'", imageName)
		}
		return nil

	case "if-not-present":
		if exists {
			return nil
		}
		// Image doesn't exist, fall through to pull

	case "always":
		// Always pull, continue

	default:
		return fmt.Errorf("invalid pull policy: %s", pullPolicy)
	}

	// Pull the image
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Read the pull output to completion (required for the pull to actually happen)
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read pull output for image %s: %w", imageName, err)
	}

	return nil
}

// imageExists checks if an image exists locally
func (c *Client) imageExists(ctx context.Context, imageName string) (bool, error) {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateContainer creates a Docker container with the specified configuration
func (c *Client) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	// Build container configuration
	containerConfig := &container.Config{
		Image:     config.Image,
		Cmd:       config.Command,
		Env:       config.Env,
		OpenStdin: config.OpenStdin,
		StdinOnce: config.StdinOnce,
		Tty:       config.Tty,
		Labels: map[string]string{
			"managed-by": "mcp-manager",
			"server":     config.Name,
		},
	}

	// Build host configuration
	hostConfig := &container.HostConfig{
		Binds:       config.Volumes,
		NetworkMode: container.NetworkMode(config.Network),
	}

	// Add restart policy if specified
	if config.RestartPolicy != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(config.RestartPolicy),
		}
	}

	// Create the container
	resp, err := c.cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		config.Name,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container %s: %w", config.Name, err)
	}

	return resp.ID, nil
}

// StartContainer starts a Docker container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}
	return nil
}

// StopContainer stops a Docker container
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout int) error {
	err := c.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, err)
	}
	return nil
}

// RemoveContainer removes a Docker container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	err := c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: force,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}
	return nil
}

// GetContainerStatus returns the status of a container
func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return &ContainerStatus{
				ID:     containerID,
				Exists: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	return &ContainerStatus{
		ID:      info.ID,
		Name:    info.Name,
		State:   info.State.Status,
		Running: info.State.Running,
		Exists:  true,
	}, nil
}

// GetContainerLogs returns logs from a container
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, options LogOptions) (io.ReadCloser, error) {
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     options.Follow,
		Tail:       fmt.Sprintf("%d", options.Tail),
	}

	reader, err := c.cli.ContainerLogs(ctx, containerID, logOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for container %s: %w", containerID, err)
	}

	return reader, nil
}

// CreateNetwork creates a Docker network if it doesn't exist
func (c *Client) CreateNetwork(ctx context.Context, networkName string) error {
	// Check if network already exists
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		if net.Name == networkName {
			return nil // Network already exists
		}
	}

	// Create the network
	_, err = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"managed-by": "mcp-manager",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", networkName, err)
	}

	return nil
}

// ListContainers lists all containers managed by mcp-manager
func (c *Client) ListContainers(ctx context.Context) ([]ContainerStatus, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var statuses []ContainerStatus
	for _, cont := range containers {
		// Filter for containers managed by mcp-manager
		if cont.Labels["managed-by"] == "mcp-manager" {
			name := cont.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:] // Remove leading slash from container name
			}

			statuses = append(statuses, ContainerStatus{
				ID:      cont.ID,
				Name:    name,
				State:   cont.State,
				Running: cont.State == "running",
				Exists:  true,
			})
		}
	}

	return statuses, nil
}

// AttachContainer attaches to a container and returns stdin/stdout streams
func (c *Client) AttachContainer(ctx context.Context, containerID string) (io.WriteCloser, io.ReadCloser, error) {
	// Attach to the container
	resp, err := c.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true, // Need to request both to properly demultiplex
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to attach to container %s: %w", containerID, err)
	}

	// Docker attach returns a multiplexed stream where stdout and stderr are mixed
	// with 8-byte headers. We need to demultiplex to extract only stdout.
	// Create a pipe to receive demultiplexed stdout
	stdoutPipe, stdoutWriter := io.Pipe()

	// Start a goroutine to demultiplex the stream
	go func() {
		// Use Docker's stdcopy to demultiplex stdout and stderr
		// We discard stderr and only keep stdout
		_, err := stdcopy.StdCopy(stdoutWriter, io.Discard, resp.Reader)
		stdoutWriter.CloseWithError(err)
	}()

	// The resp is a hijacked connection that provides both read and write
	// We need to wrap it to provide separate stdin/stdout interfaces
	stdin := &containerStdin{conn: resp}
	stdout := &containerStdout{conn: resp, reader: stdoutPipe}

	return stdin, stdout, nil
}

// containerStdin wraps the hijacked connection for writing (stdin)
type containerStdin struct {
	conn types.HijackedResponse
}

func (cs *containerStdin) Write(p []byte) (n int, err error) {
	return cs.conn.Conn.Write(p)
}

func (cs *containerStdin) Close() error {
	cs.conn.Close()
	return nil
}

// containerStdout wraps the hijacked connection for reading (stdout)
type containerStdout struct {
	conn   types.HijackedResponse
	reader io.Reader
}

func (cs *containerStdout) Read(p []byte) (n int, err error) {
	return cs.reader.Read(p)
}

func (cs *containerStdout) Close() error {
	// The connection is closed by the stdin closer
	return nil
}
