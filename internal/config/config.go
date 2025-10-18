package config

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the root configuration structure
type Config struct {
	Version  string   `yaml:"version"`
	Settings Settings `yaml:"settings"`
	Servers  []Server `yaml:"servers"`
}

// Settings contains global configuration settings
type Settings struct {
	DataDir     string `yaml:"data_dir"`
	Network     string `yaml:"network"`
	LogLevel    string `yaml:"log_level"`
	GatewayPort int    `yaml:"gateway_port"`
	GatewayMode string `yaml:"gateway_mode"`
}

// Server represents a single MCP server configuration
type Server struct {
	Name        string      `yaml:"name"`
	Enabled     bool        `yaml:"enabled"`
	Description string      `yaml:"description"`
	Transport   string      `yaml:"transport,omitempty"` // "stdio" or "http" (default: "stdio")
	URL         string      `yaml:"url,omitempty"`       // For external HTTP servers
	Port        int         `yaml:"port,omitempty"`      // For Docker containers with HTTP servers
	HTTPPath    string      `yaml:"http_path,omitempty"` // HTTP path for containers (e.g., "/mcp")
	Docker      Docker      `yaml:"docker"`
	Command     []string    `yaml:"command"`
	Args        []string    `yaml:"args,omitempty"`
	Env         []EnvVar    `yaml:"env,omitempty"`
	Volumes     []Volume    `yaml:"volumes,omitempty"`
	HealthCheck HealthCheck `yaml:"health_check"`
	Agents      []string    `yaml:"agents"`
}

// Docker contains Docker-specific configuration
type Docker struct {
	Image      string `yaml:"image"`
	PullPolicy string `yaml:"pull_policy"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name         string `yaml:"name"`
	Value        string `yaml:"value,omitempty"`
	ValueFromEnv string `yaml:"value_from_env,omitempty"`
}

// Volume represents a volume mount
type Volume struct {
	HostPath      string `yaml:"host_path"`
	ContainerPath string `yaml:"container_path"`
	ReadOnly      bool   `yaml:"read_only"`
}

// HealthCheck contains health check configuration
type HealthCheck struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Retries  int           `yaml:"retries"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate version
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate settings
	if err := c.validateSettings(); err != nil {
		return fmt.Errorf("settings validation failed: %w", err)
	}

	// Validate servers
	if err := c.validateServers(); err != nil {
		return fmt.Errorf("servers validation failed: %w", err)
	}

	return nil
}

// validateSettings validates the settings section
func (c *Config) validateSettings() error {
	s := &c.Settings

	if s.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}

	if s.Network == "" {
		return fmt.Errorf("network is required")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if s.LogLevel != "" {
		valid := false
		for _, level := range validLogLevels {
			if s.LogLevel == level {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("log_level must be one of: %s", strings.Join(validLogLevels, ", "))
		}
	}

	// Validate gateway port
	if s.GatewayPort != 0 && (s.GatewayPort < 1 || s.GatewayPort > 65535) {
		return fmt.Errorf("gateway_port must be between 1 and 65535, got: %d", s.GatewayPort)
	}

	return nil
}

// validateServers validates the servers section
func (c *Config) validateServers() error {
	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server must be defined")
	}

	// Track server names to check for duplicates
	names := make(map[string]bool)

	for i, server := range c.Servers {
		// Validate server name
		if server.Name == "" {
			return fmt.Errorf("server[%d]: name is required", i)
		}

		// Check for duplicate names
		if names[server.Name] {
			return fmt.Errorf("server[%d]: duplicate server name '%s'", i, server.Name)
		}
		names[server.Name] = true

		// Validate transport type
		if server.Transport != "" && server.Transport != "stdio" && server.Transport != "http" {
			return fmt.Errorf("server '%s': transport must be either 'stdio' or 'http', got '%s'", server.Name, server.Transport)
		}

		// Set default transport to stdio if not specified
		transport := server.Transport
		if transport == "" {
			transport = "stdio"
		}

		// Validate HTTP transport configuration
		if transport == "http" {
			// For external HTTP servers (no Docker image), URL is required
			if server.Docker.Image == "" {
				if server.URL == "" {
					return fmt.Errorf("server '%s': url is required for HTTP transport without Docker image", server.Name)
				}
			} else {
				// For Docker-based HTTP servers, Port is required
				if server.Port == 0 {
					return fmt.Errorf("server '%s': port is required for HTTP transport with Docker image", server.Name)
				}
				if server.Port < 1 || server.Port > 65535 {
					return fmt.Errorf("server '%s': port must be between 1 and 65535, got %d", server.Name, server.Port)
				}
			}
		}

		// If server is enabled, validate required fields
		if server.Enabled {
			// For stdio transport, command is required unless using Docker mode with an image (which has its own ENTRYPOINT/CMD)
			if transport == "stdio" && len(server.Command) == 0 && server.Docker.Image == "" {
				return fmt.Errorf("server '%s': command is required for enabled servers without a Docker image", server.Name)
			}
			// For HTTP transport with external URL, command/docker are optional
		}

		// Validate Docker configuration
		if err := validateDockerConfig(server.Name, &server.Docker); err != nil {
			return err
		}

		// Validate environment variables
		for j, env := range server.Env {
			if env.Name == "" {
				return fmt.Errorf("server '%s': env[%d]: name is required", server.Name, j)
			}
			// Must have either value or value_from_env, but not both
			hasValue := env.Value != ""
			hasValueFromEnv := env.ValueFromEnv != ""
			if !hasValue && !hasValueFromEnv {
				return fmt.Errorf("server '%s': env[%d] (%s): must specify either value or value_from_env", server.Name, j, env.Name)
			}
			if hasValue && hasValueFromEnv {
				return fmt.Errorf("server '%s': env[%d] (%s): cannot specify both value and value_from_env", server.Name, j, env.Name)
			}
		}

		// Validate volumes
		for j, vol := range server.Volumes {
			if vol.HostPath == "" {
				return fmt.Errorf("server '%s': volume[%d]: host_path is required", server.Name, j)
			}
			if vol.ContainerPath == "" {
				return fmt.Errorf("server '%s': volume[%d]: container_path is required", server.Name, j)
			}
		}

		// Validate health check
		if server.HealthCheck.Enabled {
			if server.HealthCheck.Interval <= 0 {
				return fmt.Errorf("server '%s': health_check interval must be > 0 when enabled", server.Name)
			}
			if server.HealthCheck.Timeout <= 0 {
				return fmt.Errorf("server '%s': health_check timeout must be > 0 when enabled", server.Name)
			}
			if server.HealthCheck.Retries < 0 {
				return fmt.Errorf("server '%s': health_check retries cannot be negative", server.Name)
			}
		}
	}

	return nil
}

// validateDockerConfig validates Docker configuration
func validateDockerConfig(serverName string, docker *Docker) error {
	// Validate pull policy if specified
	if docker.PullPolicy != "" {
		validPolicies := []string{"always", "never", "if-not-present"}
		valid := false
		for _, policy := range validPolicies {
			if docker.PullPolicy == policy {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("server '%s': pull_policy must be one of: %s", serverName, strings.Join(validPolicies, ", "))
		}
	}

	return nil
}

// GetEnabledServers returns only the enabled servers
func (c *Config) GetEnabledServers() []Server {
	var enabled []Server
	for _, server := range c.Servers {
		if server.Enabled {
			enabled = append(enabled, server)
		}
	}
	return enabled
}
