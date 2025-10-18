package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ihavespoons/mcp-manager/internal/config"
	"github.com/ihavespoons/mcp-manager/internal/gateway"
)

// Server implements an MCP server that communicates over stdio
type Server struct {
	config  *config.Config
	gateway *gateway.Gateway
	stdin   io.Reader
	stdout  io.Writer
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewServer creates a new MCP server
func NewServer(cfg *config.Config, gw *gateway.Gateway) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:  cfg,
		gateway: gw,
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Serve starts the MCP server and processes stdin messages
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.stdin)

	// Process messages line by line
	for scanner.Scan() {
		line := scanner.Bytes()

		// Parse JSON-RPC message
		var msg JSONRPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			s.sendError(0, -32700, "Parse error", err.Error())
			continue
		}

		// Handle the message
		response := s.handleMessage(&msg)

		// Send response
		if response != nil {
			s.sendResponse(response)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}

	// stdin closed, trigger graceful shutdown
	s.cancel()
	return nil
}

// handleMessage processes an MCP protocol message
func (s *Server) handleMessage(msg *JSONRPCMessage) *JSONRPCMessage {
	// Check if this is a notification (no ID field)
	// Notifications must not receive a response according to JSON-RPC 2.0
	isNotification := msg.ID == nil

	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "notifications/initialized":
		// Client is notifying us that initialization is complete
		// No response needed for notifications
		return nil
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(msg)
	default:
		// If it's a notification, don't respond
		if isNotification {
			return nil
		}
		return s.createErrorResponse(msg.ID, -32601, "Method not found", "")
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(msg *JSONRPCMessage) *JSONRPCMessage {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "mcp-manager",
			"version": "1.0.0",
		},
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
	}
}

// handleToolsList returns the list of available management tools
func (s *Server) handleToolsList(msg *JSONRPCMessage) *JSONRPCMessage {
	tools := []Tool{
		{
			Name:        "list_servers",
			Description: "List all configured MCP servers",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "server_status",
			Description: "Get the status of a specific MCP server or all servers",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server": map[string]interface{}{
						"type":        "string",
						"description": "Server name (optional, returns all if not specified)",
					},
				},
			},
		},
		{
			Name:        "gateway_status",
			Description: "Get the status of the MCP gateway",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
	}
}

// handleToolsCall executes a tool call
func (s *Server) handleToolsCall(msg *JSONRPCMessage) *JSONRPCMessage {
	// Extract params
	params, ok := msg.Params.(map[string]interface{})
	if !ok {
		return s.createErrorResponse(msg.ID, -32602, "Invalid params", "")
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.createErrorResponse(msg.ID, -32602, "Tool name required", "")
	}

	arguments, _ := params["arguments"].(map[string]interface{})

	// Execute the tool
	switch toolName {
	case "list_servers":
		return s.toolListServers(msg.ID, arguments)
	case "server_status":
		return s.toolServerStatus(msg.ID, arguments)
	case "gateway_status":
		return s.toolGatewayStatus(msg.ID, arguments)
	default:
		return s.createErrorResponse(msg.ID, -32601, "Tool not found", "")
	}
}

// toolListServers implements the list_servers tool
func (s *Server) toolListServers(id interface{}, args map[string]interface{}) *JSONRPCMessage {
	servers := s.config.Servers

	var output string
	output += fmt.Sprintf("Configured MCP Servers (%d total):\n\n", len(servers))

	for _, srv := range servers {
		status := "disabled"
		if srv.Enabled {
			status = "enabled"
		}
		output += fmt.Sprintf("• %s (%s)\n", srv.Name, status)
		if srv.Description != "" {
			output += fmt.Sprintf("  Description: %s\n", srv.Description)
		}
		output += fmt.Sprintf("  Image: %s\n", srv.Docker.Image)
		output += fmt.Sprintf("  Gateway URL: http://localhost:%d/mcp/%s\n\n",
			s.config.Settings.GatewayPort, srv.Name)
	}

	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": output,
			},
		},
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// toolServerStatus implements the server_status tool
func (s *Server) toolServerStatus(id interface{}, args map[string]interface{}) *JSONRPCMessage {
	serverName, _ := args["server"].(string)

	var output string
	if serverName != "" {
		// Get status of specific server
		for _, srv := range s.config.Servers {
			if srv.Name == serverName {
				output = fmt.Sprintf("Server: %s\n", srv.Name)
				output += fmt.Sprintf("Enabled: %v\n", srv.Enabled)
				output += fmt.Sprintf("Image: %s\n", srv.Docker.Image)
				output += fmt.Sprintf("Gateway URL: http://localhost:%d/mcp/%s\n",
					s.config.Settings.GatewayPort, srv.Name)
				break
			}
		}
		if output == "" {
			output = fmt.Sprintf("Server '%s' not found\n", serverName)
		}
	} else {
		// Get status of all servers
		output = "All Servers Status:\n\n"
		for _, srv := range s.config.Servers {
			status := "disabled"
			if srv.Enabled {
				status = "enabled"
			}
			output += fmt.Sprintf("• %s: %s\n", srv.Name, status)
		}
	}

	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": output,
			},
		},
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// toolGatewayStatus implements the gateway_status tool
func (s *Server) toolGatewayStatus(id interface{}, args map[string]interface{}) *JSONRPCMessage {
	output := fmt.Sprintf("MCP Gateway Status\n\n")
	output += fmt.Sprintf("Port: %d\n", s.config.Settings.GatewayPort)
	output += fmt.Sprintf("Mode: mixed (per-server)\n")
	output += fmt.Sprintf("  - Containers: servers with docker.image specified\n")
	output += fmt.Sprintf("  - Processes: servers without docker.image\n")
	output += fmt.Sprintf("Available Servers: %d\n", len(s.config.GetEnabledServers()))
	output += fmt.Sprintf("\nGateway Endpoints:\n")
	for _, srv := range s.config.GetEnabledServers() {
		mode := "container"
		if srv.Docker.Image == "" {
			mode = "process"
		}
		output += fmt.Sprintf("  • http://localhost:%d/mcp/%s [%s]\n",
			s.config.Settings.GatewayPort, srv.Name, mode)
	}

	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": output,
			},
		},
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// sendResponse sends a JSON-RPC response to stdout
func (s *Server) sendResponse(msg *JSONRPCMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.stdout.Write(data)
	s.stdout.Write([]byte("\n"))
}

// sendError sends a JSON-RPC error response
func (s *Server) sendError(id interface{}, code int, message string, data string) {
	response := s.createErrorResponse(id, code, message, data)
	s.sendResponse(response)
}

// createErrorResponse creates a JSON-RPC error response
func (s *Server) createErrorResponse(id interface{}, code int, message string, data string) *JSONRPCMessage {
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if data != "" {
		errorObj["data"] = data
	}

	return &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   errorObj,
	}
}

// Stop gracefully stops the MCP server
func (s *Server) Stop() {
	s.cancel()
}
