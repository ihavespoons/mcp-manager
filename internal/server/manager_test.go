package server

import (
	"strings"
	"testing"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
)

func TestNewManager(t *testing.T) {
	dockerClient := &docker.Client{}
	cfg := &config.Config{}

	manager := NewManager(dockerClient, cfg)

	if manager == nil {
		t.Fatal("expected manager to be non-nil")
	}

	if manager.docker != dockerClient {
		t.Error("expected docker client to be set correctly")
	}

	if manager.config != cfg {
		t.Error("expected config to be set correctly")
	}
}

func TestBuildEnv(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name     string
		envVars  []config.EnvVar
		wantLen  int
		wantErr  bool
		contains string
	}{
		{
			name: "simple value",
			envVars: []config.EnvVar{
				{Name: "TEST", Value: "value"},
			},
			wantLen:  1,
			wantErr:  false,
			contains: "TEST=value",
		},
		{
			name: "multiple values",
			envVars: []config.EnvVar{
				{Name: "VAR1", Value: "value1"},
				{Name: "VAR2", Value: "value2"},
			},
			wantLen:  2,
			wantErr:  false,
			contains: "VAR1=value1",
		},
		{
			name:    "empty",
			envVars: []config.EnvVar{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := manager.buildEnv(tt.envVars)

			if (err != nil) != tt.wantErr {
				t.Errorf("buildEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(env) != tt.wantLen {
				t.Errorf("buildEnv() returned %d items, want %d", len(env), tt.wantLen)
			}

			if tt.contains != "" {
				found := false
				for _, e := range env {
					if e == tt.contains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildEnv() result does not contain %s", tt.contains)
				}
			}
		})
	}
}

func TestBuildVolumes(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name     string
		volumes  []config.Volume
		wantLen  int
		wantErr  bool
		contains string
	}{
		{
			name: "simple volume",
			volumes: []config.Volume{
				{HostPath: "/host", ContainerPath: "/container", ReadOnly: false},
			},
			wantLen:  1,
			wantErr:  false,
			contains: "/host:/container",
		},
		{
			name: "readonly volume",
			volumes: []config.Volume{
				{HostPath: "/host", ContainerPath: "/container", ReadOnly: true},
			},
			wantLen:  1,
			wantErr:  false,
			contains: "/host:/container:ro",
		},
		{
			name: "tilde expansion",
			volumes: []config.Volume{
				{HostPath: "~/.gitconfig", ContainerPath: "/root/.gitconfig", ReadOnly: true},
			},
			wantLen: 1,
			wantErr: false,
			// We can't check exact path since it depends on the user's home dir
		},
		{
			name: "multiple volumes",
			volumes: []config.Volume{
				{HostPath: "/host1", ContainerPath: "/container1", ReadOnly: false},
				{HostPath: "/host2", ContainerPath: "/container2", ReadOnly: true},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "empty",
			volumes: []config.Volume{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumes, err := manager.buildVolumes(tt.volumes)

			if (err != nil) != tt.wantErr {
				t.Errorf("buildVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(volumes) != tt.wantLen {
				t.Errorf("buildVolumes() returned %d items, want %d", len(volumes), tt.wantLen)
			}

			if tt.contains != "" {
				found := false
				for _, v := range volumes {
					if v == tt.contains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildVolumes() result does not contain %s", tt.contains)
				}
			}

			// Special check for tilde expansion test
			if tt.name == "tilde expansion" && len(volumes) > 0 {
				// Should not contain tilde anymore
				if strings.Contains(volumes[0], "~") {
					t.Errorf("buildVolumes() failed to expand tilde: %s", volumes[0])
				}
				// Should contain :ro flag
				if !strings.HasSuffix(volumes[0], ":ro") {
					t.Errorf("buildVolumes() missing readonly flag: %s", volumes[0])
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{
				Name:        "server1",
				Description: "Test Server 1",
				Enabled:     true,
				Docker: config.Docker{
					Image: "test/image1:latest",
				},
			},
			{
				Name:        "server2",
				Description: "Test Server 2",
				Enabled:     false,
				Docker: config.Docker{
					Image: "test/image2:latest",
				},
			},
		},
	}

	manager := NewManager(nil, cfg)
	servers := manager.List()

	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}

	if servers[0].Name != "server1" {
		t.Errorf("expected first server name to be 'server1', got %s", servers[0].Name)
	}

	if servers[0].Enabled != true {
		t.Error("expected first server to be enabled")
	}

	if servers[1].Enabled != false {
		t.Error("expected second server to be disabled")
	}
}

func TestFindServer(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{Name: "server1"},
			{Name: "server2"},
		},
	}

	manager := NewManager(nil, cfg)

	tests := []struct {
		name       string
		searchName string
		wantFound  bool
	}{
		{
			name:       "exact match",
			searchName: "server1",
			wantFound:  true,
		},
		{
			name:       "case insensitive match",
			searchName: "SERVER1",
			wantFound:  true,
		},
		{
			name:       "not found",
			searchName: "server3",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := manager.findServer(tt.searchName)
			found := server != nil

			if found != tt.wantFound {
				t.Errorf("findServer(%s) found = %v, want %v", tt.searchName, found, tt.wantFound)
			}
		})
	}
}
