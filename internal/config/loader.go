package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPaths returns the default locations to search for config files
func DefaultConfigPaths() []string {
	homeDir, _ := os.UserHomeDir()
	return []string{
		"./mcp-config.yaml",
		"./mcp-config.yml",
		"./.mcp-manager/config.yaml",
		"./.mcp-manager/config.yml",
		filepath.Join(homeDir, ".mcp-manager", "config.yaml"),
		filepath.Join(homeDir, ".mcp-manager", "config.yml"),
	}
}

// Load loads and parses the configuration file
func Load(path string) (*Config, error) {
	// If no path is provided, search default locations
	if path == "" {
		var err error
		path, err = findConfig()
		if err != nil {
			return nil, err
		}
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config
	expandedData := []byte(os.ExpandEnv(string(data)))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(expandedData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// findConfig searches for a config file in default locations
func findConfig() (string, error) {
	for _, path := range DefaultConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no configuration file found in default locations")
}

// setDefaults sets default values for configuration fields
func setDefaults(config *Config) {
	// Set default settings
	if config.Settings.DataDir == "" {
		homeDir, _ := os.UserHomeDir()
		config.Settings.DataDir = filepath.Join(homeDir, ".mcp-manager")
	} else {
		// Expand ~ to home directory
		if strings.HasPrefix(config.Settings.DataDir, "~/") {
			homeDir, _ := os.UserHomeDir()
			config.Settings.DataDir = filepath.Join(homeDir, config.Settings.DataDir[2:])
		}
	}

	if config.Settings.Network == "" {
		config.Settings.Network = "mcp-network"
	}

	if config.Settings.LogLevel == "" {
		config.Settings.LogLevel = "info"
	}

	if config.Settings.GatewayPort == 0 {
		config.Settings.GatewayPort = 52080
	}

	// Set defaults for each server
	for i := range config.Servers {
		server := &config.Servers[i]

		if server.Docker.PullPolicy == "" {
			server.Docker.PullPolicy = "if-not-present"
		}

		if !server.HealthCheck.Enabled {
			// Set default health check values if not specified
			if server.HealthCheck.Interval == 0 {
				server.HealthCheck.Interval = 30
			}
			if server.HealthCheck.Timeout == 0 {
				server.HealthCheck.Timeout = 10
			}
			if server.HealthCheck.Retries == 0 {
				server.HealthCheck.Retries = 3
			}
		}
	}
}

// Save writes the configuration to a file
func Save(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
