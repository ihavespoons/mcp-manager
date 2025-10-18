package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ihavespoons/mcp-manager/internal/config"
)

func TestServer_handleInitialize(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir: "/data",
			Network: "test",
		},
		Servers: []config.Server{},
	}

	srv := &Server{
		config: cfg,
	}

	msg := &JSONRPCMessage{
		ID:     1,
		Method: "initialize",
	}

	response := srv.handleInitialize(msg)

	if response == nil {
		t.Fatal("handleInitialize returned nil")
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", response.JSONRPC)
	}

	if response.Result == nil {
		t.Error("expected result to be non-nil")
	}

	// Check result structure
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("serverInfo is not a map")
	}

	if serverInfo["name"] != "mcp-manager" {
		t.Errorf("expected server name mcp-manager, got %v", serverInfo["name"])
	}
}

func TestServer_handleToolsList(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir: "/data",
			Network: "test",
		},
		Servers: []config.Server{},
	}

	srv := &Server{
		config: cfg,
	}

	msg := &JSONRPCMessage{
		ID:     1,
		Method: "tools/list",
	}

	response := srv.handleToolsList(msg)

	if response == nil {
		t.Fatal("handleToolsList returned nil")
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	tools, ok := result["tools"].([]Tool)
	if !ok {
		t.Fatal("tools is not a slice")
	}

	// Should have 3 tools: list_servers, server_status, gateway_status
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	// Check tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"list_servers", "server_status", "gateway_status"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestServer_toolListServers(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir:     "/data",
			Network:     "test",
			GatewayPort: 8080,
		},
		Servers: []config.Server{
			{
				Name:        "test-server1",
				Enabled:     true,
				Description: "Test Server 1",
			},
			{
				Name:    "test-server2",
				Enabled: false,
			},
		},
	}

	srv := &Server{
		config: cfg,
	}

	response := srv.toolListServers(1, nil)

	if response == nil {
		t.Fatal("toolListServers returned nil")
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	content, ok := result["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("content is not a slice")
	}

	if len(content) == 0 {
		t.Fatal("content is empty")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("text is not a string")
	}

	// Check that output contains server info
	if !strings.Contains(text, "test-server1") {
		t.Error("output missing test-server1")
	}
	if !strings.Contains(text, "test-server2") {
		t.Error("output missing test-server2")
	}
	if !strings.Contains(text, "enabled") {
		t.Error("output missing status")
	}
}

func TestServer_toolServerStatus(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir:     "/data",
			Network:     "test",
			GatewayPort: 8080,
		},
		Servers: []config.Server{
			{
				Name:    "test-server",
				Enabled: true,
			},
		},
	}

	srv := &Server{
		config: cfg,
	}

	t.Run("specific server", func(t *testing.T) {
		args := map[string]interface{}{
			"server": "test-server",
		}

		response := srv.toolServerStatus(1, args)

		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatal("result is not a map")
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("content is not a slice")
		}

		text, ok := content[0]["text"].(string)
		if !ok {
			t.Fatal("text is not a string")
		}

		if !strings.Contains(text, "test-server") {
			t.Error("output missing server name")
		}
		if !strings.Contains(text, "Enabled: true") {
			t.Error("output missing enabled status")
		}
	})

	t.Run("all servers", func(t *testing.T) {
		response := srv.toolServerStatus(1, nil)

		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatal("result is not a map")
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("content is not a slice")
		}

		text, ok := content[0]["text"].(string)
		if !ok {
			t.Fatal("text is not a string")
		}

		if !strings.Contains(text, "All Servers Status") {
			t.Error("output missing title")
		}
		if !strings.Contains(text, "test-server") {
			t.Error("output missing server name")
		}
	})

	t.Run("server not found", func(t *testing.T) {
		args := map[string]interface{}{
			"server": "nonexistent",
		}

		response := srv.toolServerStatus(1, args)

		result, ok := response.Result.(map[string]interface{})
		if !ok {
			t.Fatal("result is not a map")
		}

		content, ok := result["content"].([]map[string]interface{})
		if !ok {
			t.Fatal("content is not a slice")
		}

		text, ok := content[0]["text"].(string)
		if !ok {
			t.Fatal("text is not a string")
		}

		if !strings.Contains(text, "not found") {
			t.Error("output should indicate server not found")
		}
	})
}

func TestServer_toolGatewayStatus(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir:     "/data",
			Network:     "test",
			GatewayPort: 8080,
		},
		Servers: []config.Server{
			{Name: "server1", Enabled: true},
			{Name: "server2", Enabled: true},
			{Name: "server3", Enabled: false},
		},
	}

	srv := &Server{
		config: cfg,
	}

	response := srv.toolGatewayStatus(1, nil)

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	content, ok := result["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("content is not a slice")
	}

	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatal("text is not a string")
	}

	// Check for expected information
	if !strings.Contains(text, "Port: 8080") {
		t.Error("output missing port")
	}
	if !strings.Contains(text, "Available Servers: 2") {
		t.Error("output should show 2 enabled servers")
	}
	if !strings.Contains(text, "server1") {
		t.Error("output missing enabled server1")
	}
	if !strings.Contains(text, "server2") {
		t.Error("output missing enabled server2")
	}
}

func TestServer_handleMessage(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir: "/data",
			Network: "test",
		},
		Servers: []config.Server{},
	}

	srv := &Server{
		config: cfg,
	}

	tests := []struct {
		name       string
		msg        *JSONRPCMessage
		wantMethod string
		wantError  bool
	}{
		{
			name: "initialize",
			msg: &JSONRPCMessage{
				ID:     1,
				Method: "initialize",
			},
			wantMethod: "initialize",
			wantError:  false,
		},
		{
			name: "tools/list",
			msg: &JSONRPCMessage{
				ID:     1,
				Method: "tools/list",
			},
			wantMethod: "tools/list",
			wantError:  false,
		},
		{
			name: "unknown method",
			msg: &JSONRPCMessage{
				ID:     1,
				Method: "unknown",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := srv.handleMessage(tt.msg)

			if response == nil {
				t.Fatal("handleMessage returned nil")
			}

			if tt.wantError {
				if response.Error == nil {
					t.Error("expected error response")
				}
			} else {
				if response.Error != nil {
					t.Errorf("unexpected error: %v", response.Error)
				}
				if response.Result == nil {
					t.Error("expected result to be non-nil")
				}
			}
		})
	}
}

func TestServer_handleToolsCall(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Settings: config.Settings{
			DataDir: "/data",
			Network: "test",
		},
		Servers: []config.Server{},
	}

	srv := &Server{
		config: cfg,
	}

	t.Run("valid tool call", func(t *testing.T) {
		params := map[string]interface{}{
			"name":      "list_servers",
			"arguments": map[string]interface{}{},
		}

		msg := &JSONRPCMessage{
			ID:     1,
			Method: "tools/call",
			Params: params,
		}

		response := srv.handleToolsCall(msg)

		if response.Error != nil {
			t.Errorf("unexpected error: %v", response.Error)
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		params := map[string]interface{}{
			"name":      "unknown_tool",
			"arguments": map[string]interface{}{},
		}

		msg := &JSONRPCMessage{
			ID:     1,
			Method: "tools/call",
			Params: params,
		}

		response := srv.handleToolsCall(msg)

		if response.Error == nil {
			t.Error("expected error for unknown tool")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		msg := &JSONRPCMessage{
			ID:     1,
			Method: "tools/call",
			Params: "invalid",
		}

		response := srv.handleToolsCall(msg)

		if response.Error == nil {
			t.Error("expected error for invalid params")
		}
	})
}

func TestServer_sendResponse(t *testing.T) {
	cfg := &config.Config{
		Version:  "1.0",
		Settings: config.Settings{DataDir: "/data", Network: "test"},
		Servers:  []config.Server{},
	}

	buf := &bytes.Buffer{}
	srv := &Server{
		config: cfg,
		stdout: buf,
	}

	msg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]interface{}{"success": true},
	}

	srv.sendResponse(msg)

	output := buf.String()
	if !strings.Contains(output, `"jsonrpc":"2.0"`) {
		t.Error("output missing jsonrpc field")
	}
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes()[:len(buf.Bytes())-1], &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestServer_createErrorResponse(t *testing.T) {
	cfg := &config.Config{
		Version:  "1.0",
		Settings: config.Settings{DataDir: "/data", Network: "test"},
		Servers:  []config.Server{},
	}

	srv := &Server{
		config: cfg,
	}

	response := srv.createErrorResponse(1, -32600, "Invalid Request", "additional data")

	if response.Error == nil {
		t.Fatal("expected error to be non-nil")
	}

	errorMap, ok := response.Error.(map[string]interface{})
	if !ok {
		t.Fatal("error is not a map")
	}

	if code, ok := errorMap["code"].(int); !ok || code != -32600 {
		t.Errorf("expected code -32600, got %v", errorMap["code"])
	}

	if msg, ok := errorMap["message"].(string); !ok || msg != "Invalid Request" {
		t.Errorf("expected message 'Invalid Request', got %v", errorMap["message"])
	}

	if data, ok := errorMap["data"].(string); !ok || data != "additional data" {
		t.Errorf("expected data 'additional data', got %v", errorMap["data"])
	}
}
