package config

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir:     "/data",
					Network:     "mcp-network",
					LogLevel:    "info",
					GatewayPort: 8080,
				},
				Servers: []Server{
					{
						Name:    "test-server",
						Enabled: true,
						Command: []string{"test"},
						Docker: Docker{
							PullPolicy: "if-not-present",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			config: Config{
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "missing data_dir",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					Network: "mcp-network",
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "data_dir is required",
		},
		{
			name: "invalid log level",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir:  "/data",
					Network:  "mcp-network",
					LogLevel: "invalid",
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "log_level must be one of",
		},
		{
			name: "invalid gateway port",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir:     "/data",
					Network:     "mcp-network",
					GatewayPort: 99999,
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "gateway_port must be between 1 and 65535",
		},
		{
			name: "no servers defined",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{},
			},
			wantErr: true,
			errMsg:  "at least one server must be defined",
		},
		{
			name: "server missing name",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{Name: "", Enabled: true, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "duplicate server names",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{"test"}},
					{Name: "test", Enabled: false, Command: []string{"test"}},
				},
			},
			wantErr: true,
			errMsg:  "duplicate server name",
		},
		{
			name: "enabled server missing command",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{Name: "test", Enabled: true, Command: []string{}},
				},
			},
			wantErr: true,
			errMsg:  "command is required for enabled servers",
		},
		{
			name: "invalid pull policy",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Docker: Docker{
							PullPolicy: "invalid",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "pull_policy must be one of",
		},
		{
			name: "env var missing name",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Env: []EnvVar{
							{Name: "", Value: "test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "env var missing value",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Env: []EnvVar{
							{Name: "TEST"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "must specify either value or value_from_env",
		},
		{
			name: "env var both value and value_from_env",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Env: []EnvVar{
							{Name: "TEST", Value: "val", ValueFromEnv: "HOST_VAR"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "cannot specify both value and value_from_env",
		},
		{
			name: "volume missing host_path",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Volumes: []Volume{
							{HostPath: "", ContainerPath: "/test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "host_path is required",
		},
		{
			name: "volume missing container_path",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						Volumes: []Volume{
							{HostPath: "/test", ContainerPath: ""},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "container_path is required",
		},
		{
			name: "health check invalid interval",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						HealthCheck: HealthCheck{
							Enabled:  true,
							Interval: 0,
							Timeout:  10 * time.Second,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "health_check interval must be > 0 when enabled",
		},
		{
			name: "health check invalid timeout",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						HealthCheck: HealthCheck{
							Enabled:  true,
							Interval: 30 * time.Second,
							Timeout:  0,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "health_check timeout must be > 0 when enabled",
		},
		{
			name: "health check negative retries",
			config: Config{
				Version: "1.0",
				Settings: Settings{
					DataDir: "/data",
					Network: "mcp-network",
				},
				Servers: []Server{
					{
						Name:    "test",
						Enabled: true,
						Command: []string{"test"},
						HealthCheck: HealthCheck{
							Enabled:  true,
							Interval: 30 * time.Second,
							Timeout:  10 * time.Second,
							Retries:  -1,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "health_check retries cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Config.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestConfig_GetEnabledServers(t *testing.T) {
	config := Config{
		Servers: []Server{
			{Name: "server1", Enabled: true},
			{Name: "server2", Enabled: false},
			{Name: "server3", Enabled: true},
			{Name: "server4", Enabled: false},
		},
	}

	enabled := config.GetEnabledServers()
	if len(enabled) != 2 {
		t.Errorf("GetEnabledServers() returned %d servers, want 2", len(enabled))
	}

	if enabled[0].Name != "server1" || enabled[1].Name != "server3" {
		t.Errorf("GetEnabledServers() returned wrong servers")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsRecursive(s, substr)))
}

func containsRecursive(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsRecursive(s[1:], substr)
}
