package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ihavespoons/mcp-manager/internal/config"
	"github.com/ihavespoons/mcp-manager/internal/log"
)

// ServerInstance represents a running MCP server instance
type ServerInstance struct {
	ServerName  string
	Transport   string // "stdio" or "http"
	ContainerID string
	ProcessID   int
	// stdio fields
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	messageWriter *MessageWriter
	messageReader *MessageReader
	// HTTP fields
	httpClient *http.Client
	httpURL    string
	Created    time.Time
	gateway    *Gateway // Reference to gateway for cleanup
	mu         sync.RWMutex
}

// ServerManager manages running MCP server instances
type ServerManager struct {
	servers map[string]*ServerInstance // Key: server name
	mu      sync.RWMutex
}

// NewServerManager creates a new server manager
func NewServerManager() *ServerManager {
	return &ServerManager{
		servers: make(map[string]*ServerInstance),
	}
}

// GetOrCreate gets an existing server instance or creates a new one
func (sm *ServerManager) GetOrCreate(serverName string, gateway *Gateway) (*ServerInstance, error) {
	log.Debug("GetOrCreate called", map[string]interface{}{
		"server_name": serverName,
	})

	// Check if server already exists and is healthy
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if instance, exists := sm.servers[serverName]; exists {
		// Check if instance is still usable based on transport type
		isUsable := false
		if instance.Transport == "http" {
			// For HTTP transport, check if httpClient and httpURL are set
			isUsable = instance.httpClient != nil && instance.httpURL != ""
		} else {
			// For stdio transport, check if stdin/stdout are set
			isUsable = instance.stdin != nil && instance.stdout != nil
		}

		if isUsable {
			log.Debug("Reusing existing server instance", map[string]interface{}{
				"server_name":  serverName,
				"container_id": instance.ContainerID,
				"process_id":   instance.ProcessID,
				"transport":    instance.Transport,
			})
			return instance, nil
		}
		// Instance is stale, remove it
		log.Warn("Removing stale server instance", map[string]interface{}{
			"server_name":  serverName,
			"container_id": instance.ContainerID,
			"transport":    instance.Transport,
		})
		instance.Stop()
		delete(sm.servers, serverName)
	}
	// Continue to create new instance (lock already held)

	log.Info("Creating new server instance", map[string]interface{}{
		"server_name": serverName,
	})

	instance := &ServerInstance{
		ServerName: serverName,
		Created:    time.Now(),
		gateway:    gateway,
	}

	// Find server config
	var serverConfig *config.Server
	for i, s := range gateway.config.Servers {
		if s.Name == serverName && s.Enabled {
			serverConfig = &gateway.config.Servers[i]
			break
		}
	}

	if serverConfig == nil {
		log.Error("Server not found or not enabled", map[string]interface{}{
			"server_name": serverName,
		})
		return nil, fmt.Errorf("server %s not found or not enabled", serverName)
	}

	// Determine transport type (default to stdio if not specified)
	transport := serverConfig.Transport
	if transport == "" {
		transport = "stdio"
	}
	instance.Transport = transport

	// Determine spawn mode based on transport and configuration
	var err error
	useContainer := serverConfig.Docker.Image != ""

	log.Debug("Determined spawn mode", map[string]interface{}{
		"server_name":   serverName,
		"transport":     transport,
		"use_container": useContainer,
		"docker_image":  serverConfig.Docker.Image,
	})

	if transport == "http" {
		// HTTP transport
		if useContainer {
			// HTTP server in Docker container
			err = instance.spawnHTTPContainer(context.Background(), gateway, *serverConfig)
		} else {
			// External HTTP endpoint
			err = instance.connectHTTPExternal(context.Background(), *serverConfig)
		}
	} else {
		// stdio transport
		if useContainer {
			err = instance.spawnContainer(context.Background(), gateway, *serverConfig)
		} else {
			err = instance.spawnProcess(context.Background(), gateway, *serverConfig)
		}
	}

	if err != nil {
		log.Error("Failed to spawn server", map[string]interface{}{
			"server_name":   serverName,
			"use_container": useContainer,
			"error":         err.Error(),
		})
		return nil, fmt.Errorf("failed to spawn server: %w", err)
	}

	log.Info("Server instance created successfully", map[string]interface{}{
		"server_name":  serverName,
		"container_id": instance.ContainerID,
		"process_id":   instance.ProcessID,
	})

	sm.servers[serverName] = instance
	return instance, nil
}

// Get retrieves a server instance by name
func (sm *ServerManager) Get(serverName string) *ServerInstance {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.servers[serverName]
}

// Delete removes a server instance
func (sm *ServerManager) Delete(serverName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if instance := sm.servers[serverName]; instance != nil {
		instance.Stop()
	}
	delete(sm.servers, serverName)
}

// Count returns the number of running servers
func (sm *ServerManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.servers)
}

// StopAll stops all running server instances
func (sm *ServerManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, instance := range sm.servers {
		instance.Stop()
	}
	sm.servers = make(map[string]*ServerInstance)
}

// SendMessage sends a message to the MCP server and returns the response
func (s *ServerInstance) SendMessage(ctx context.Context, body io.Reader) ([]byte, error) {
	log.Debug("SendMessage: Acquiring lock", map[string]interface{}{
		"server_name": s.ServerName,
		"transport":   s.Transport,
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Debug("SendMessage: Lock acquired", map[string]interface{}{
		"server_name": s.ServerName,
	})

	// Read the request body
	requestData, err := io.ReadAll(body)
	if err != nil {
		log.Error("SendMessage: Failed to read request body", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	log.Debug("SendMessage: Read request body", map[string]interface{}{
		"server_name": s.ServerName,
		"bytes":       len(requestData),
	})

	// Route based on transport type
	if s.Transport == "http" {
		return s.sendHTTPMessage(ctx, requestData)
	}
	return s.sendStdioMessage(ctx, requestData)
}

// sendStdioMessage sends a message via stdio transport
func (s *ServerInstance) sendStdioMessage(ctx context.Context, requestData []byte) ([]byte, error) {
	// Parse as JSON-RPC message
	msg, err := ParseRequest(requestData)
	if err != nil {
		log.Error("SendMessage: Failed to parse JSON-RPC request", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}
	log.Debug("SendMessage: Parsed JSON-RPC message", map[string]interface{}{
		"server_name": s.ServerName,
	})

	// Check if this is a notification (no ID field = no response expected)
	isNotification := msg.ID == nil

	// Write message to server stdin with newline delimiter
	log.Debug("SendMessage: Writing message to server stdin", map[string]interface{}{
		"server_name":     s.ServerName,
		"is_notification": isNotification,
	})
	if err := s.messageWriter.WriteMessage(msg); err != nil {
		log.Error("SendMessage: Failed to write to server", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to write to server: %w", err)
	}

	// If this is a notification, don't wait for a response
	if isNotification {
		log.Info("SendMessage: Notification sent (no response expected)", map[string]interface{}{
			"server_name": s.ServerName,
			"method":      msg.Method,
		})
		// Return an empty success response for notifications
		return []byte(`{}`), nil
	}

	log.Debug("SendMessage: Message written, waiting for response", map[string]interface{}{
		"server_name": s.ServerName,
	})

	// Read response from server stdout
	response, err := s.messageReader.ReadMessage()
	if err != nil {
		log.Error("SendMessage: Failed to read from server", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to read from server: %w", err)
	}
	log.Debug("SendMessage: Received response from server", map[string]interface{}{
		"server_name": s.ServerName,
	})

	// Serialize response back to JSON (without newline for HTTP response)
	responseData, err := SerializeMessage(response)
	if err != nil {
		log.Error("SendMessage: Failed to serialize response", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to serialize response: %w", err)
	}

	log.Info("SendMessage: Complete", map[string]interface{}{
		"server_name":    s.ServerName,
		"response_bytes": len(responseData),
	})

	return responseData, nil
}

// sendHTTPMessage sends a message via HTTP transport
func (s *ServerInstance) sendHTTPMessage(ctx context.Context, requestData []byte) ([]byte, error) {
	log.Debug("SendHTTPMessage: Sending HTTP POST request", map[string]interface{}{
		"server_name": s.ServerName,
		"url":         s.httpURL,
		"bytes":       len(requestData),
	})

	// Create HTTP POST request
	req, err := http.NewRequestWithContext(ctx, "POST", s.httpURL, bytes.NewReader(requestData))
	if err != nil {
		log.Error("SendHTTPMessage: Failed to create HTTP request", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers for JSON-RPC
	req.Header.Set("Content-Type", "application/json")
	// Some MCP servers (like context7) require Accept header for both JSON and SSE
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Send the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Error("SendHTTPMessage: HTTP request failed", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code - accept any 2xx status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error("SendHTTPMessage: Bad HTTP status", map[string]interface{}{
			"server_name": s.ServerName,
			"status_code": resp.StatusCode,
		})
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	// Read response
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("SendHTTPMessage: Failed to read HTTP response", map[string]interface{}{
			"server_name": s.ServerName,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to read HTTP response: %w", err)
	}

	// Check if response is SSE format (Server-Sent Events)
	// SSE responses have format: "event: message\ndata: {...}\n"
	responseStr := string(responseData)
	if strings.HasPrefix(responseStr, "event:") {
		// Parse SSE format - extract JSON from "data: " line
		lines := strings.Split(responseStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				// Extract JSON data after "data: " prefix
				jsonData := strings.TrimPrefix(line, "data: ")
				responseData = []byte(jsonData)
				log.Debug("SendHTTPMessage: Parsed SSE response", map[string]interface{}{
					"server_name": s.ServerName,
					"json_bytes":  len(responseData),
				})
				break
			}
		}
	}

	log.Info("SendHTTPMessage: Complete", map[string]interface{}{
		"server_name":    s.ServerName,
		"response_bytes": len(responseData),
	})

	return responseData, nil
}

// Stop stops the server instance and cleans up resources
func (s *ServerInstance) Stop() {
	// Close pipes
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}

	// Cleanup Docker container if present
	if s.ContainerID != "" && s.gateway != nil && s.gateway.docker != nil {
		ctx := context.Background()
		// Force remove the container (stops and removes in one operation)
		s.gateway.docker.RemoveContainer(ctx, s.ContainerID, true)
	}
}
