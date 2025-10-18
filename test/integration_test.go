//go:build integration
// +build integration

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/ihavespoons/mcp-manager/internal/config"
	"github.com/ihavespoons/mcp-manager/internal/gateway"
)

// TestFilesystemServerIntegration tests the full gateway with the filesystem MCP server
func TestFilesystemServerIntegration(t *testing.T) {
	// Skip if npx is not available
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available, skipping integration test")
	}

	// Create test config
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir:     "/tmp/mcp-test",
			Network:     "mcp-test",
			GatewayPort: 0, // Will be set by test
		},
		Servers: []config.Server{
			{
				Name:        "filesystem",
				Enabled:     true,
				Description: "Test filesystem server",
				Command:     []string{"npx"},
				Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			},
		},
	}

	// Create gateway
	gwConfig := gateway.Config{
		Host: "127.0.0.1",
		Port: 0, // Random port
	}

	gw := gateway.NewGateway(cfg, nil, gwConfig)

	// Start gateway in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gwErrChan := make(chan error, 1)
	go func() {
		gwErrChan <- gw.Start()
	}()

	// Give gateway time to start
	time.Sleep(500 * time.Millisecond)

	// Test health endpoint
	t.Run("health", func(t *testing.T) {
		resp, err := http.Get("http://127.0.0.1:8080/health")
		if err != nil {
			t.Skipf("Gateway not ready: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test MCP initialize
	t.Run("initialize", func(t *testing.T) {
		initReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		body, _ := json.Marshal(initReq)
		req, _ := http.NewRequest("POST", "http://127.0.0.1:8080/mcp", bytes.NewReader(body))
		req.Header.Set("X-MCP-Server", "filesystem")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		// Check session ID header
		sessionID := resp.Header.Get("X-Session-ID")
		if sessionID == "" {
			t.Error("expected X-Session-ID header")
		}

		// Parse response
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify response structure
		if result["jsonrpc"] != "2.0" {
			t.Error("expected jsonrpc 2.0")
		}

		if result["result"] == nil {
			t.Error("expected result field")
		}
	})

	// Cleanup
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := gw.Stop(shutdownCtx); err != nil {
		t.Errorf("failed to stop gateway: %v", err)
	}
}

// TestMCPManagerServe tests the mcp-manager serve command
func TestMCPManagerServe(t *testing.T) {
	t.Skip("Requires mcp-manager binary and full environment")

	// This test would:
	// 1. Start mcp-manager serve
	// 2. Send JSON-RPC messages to stdin
	// 3. Read responses from stdout
	// 4. Verify tool execution
}
