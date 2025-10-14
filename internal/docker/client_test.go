package docker

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	// This test requires Docker to be running
	client, err := NewClient()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("expected client to be non-nil")
	}

	if client.cli == nil {
		t.Fatal("expected client.cli to be non-nil")
	}
}

func TestPullPolicyLogic(t *testing.T) {
	tests := []struct {
		name       string
		pullPolicy string
		wantError  bool
	}{
		{
			name:       "valid policy: always",
			pullPolicy: "always",
			wantError:  false,
		},
		{
			name:       "valid policy: never",
			pullPolicy: "never",
			wantError:  false,
		},
		{
			name:       "valid policy: if-not-present",
			pullPolicy: "if-not-present",
			wantError:  false,
		},
		{
			name:       "invalid policy",
			pullPolicy: "invalid",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We're just testing the validation logic here
			validPolicies := map[string]bool{
				"always":         true,
				"never":          true,
				"if-not-present": true,
			}

			_, ok := validPolicies[tt.pullPolicy]
			if ok && tt.wantError {
				t.Errorf("expected error for policy %s, but got none", tt.pullPolicy)
			}
			if !ok && !tt.wantError {
				t.Errorf("expected no error for policy %s, but got error", tt.pullPolicy)
			}
		})
	}
}

func TestContainerConfig(t *testing.T) {
	config := &ContainerConfig{
		Name:    "test-container",
		Image:   "alpine:latest",
		Command: []string{"/bin/sh"},
		Env:     []string{"TEST=value"},
		Volumes: []string{"/host:/container"},
		Network: "test-network",
	}

	if config.Name != "test-container" {
		t.Errorf("expected Name to be 'test-container', got %s", config.Name)
	}

	if config.Image != "alpine:latest" {
		t.Errorf("expected Image to be 'alpine:latest', got %s", config.Image)
	}

	if len(config.Env) != 1 {
		t.Errorf("expected 1 environment variable, got %d", len(config.Env))
	}
}

func TestContainerStatus(t *testing.T) {
	status := &ContainerStatus{
		ID:      "abc123",
		Name:    "test-container",
		State:   "running",
		Running: true,
		Exists:  true,
	}

	if !status.Running {
		t.Error("expected Running to be true")
	}

	if !status.Exists {
		t.Error("expected Exists to be true")
	}

	if status.State != "running" {
		t.Errorf("expected State to be 'running', got %s", status.State)
	}
}

func TestLogOptions(t *testing.T) {
	opts := LogOptions{
		Follow: true,
		Tail:   100,
	}

	if !opts.Follow {
		t.Error("expected Follow to be true")
	}

	if opts.Tail != 100 {
		t.Errorf("expected Tail to be 100, got %d", opts.Tail)
	}
}
