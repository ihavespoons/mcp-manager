package config

import (
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
}

// Server represents a single MCP server configuration
type Server struct {
	Name        string      `yaml:"name"`
	Enabled     bool        `yaml:"enabled"`
	Description string      `yaml:"description"`
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
	// TODO: Implement validation logic
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
