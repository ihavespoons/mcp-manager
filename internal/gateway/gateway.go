package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
	"github.com/bengittins/mcp-manager/internal/log"
)

// Gateway manages MCP client connections and server lifecycle
type Gateway struct {
	config  *config.Config
	docker  *docker.Client
	host    string
	port    int
	server  *http.Server
	servers *ServerManager
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// Config contains gateway configuration
type Config struct {
	Host string
	Port int
}

// NewGateway creates a new MCP gateway
func NewGateway(cfg *config.Config, dockerClient *docker.Client, gwConfig Config) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())

	gw := &Gateway{
		config:  cfg,
		docker:  dockerClient,
		host:    gwConfig.Host,
		port:    gwConfig.Port,
		servers: NewServerManager(),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/", gw.handleMCP) // Path-based routing: /mcp/{serverName}
	mux.HandleFunc("/mcp", gw.handleMCP)  // Legacy: /mcp with X-MCP-Server header
	mux.HandleFunc("/health", gw.handleHealth)
	mux.HandleFunc("/servers", gw.handleServers)

	gw.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", gw.host, gw.port),
		Handler: mux,
	}

	return gw
}

// Start starts the gateway HTTP server
func (g *Gateway) Start() error {
	// Initialize structured logging based on config level
	logLevel := log.INFO
	switch g.config.Settings.LogLevel {
	case "debug":
		logLevel = log.DEBUG
	case "info":
		logLevel = log.INFO
	case "warn":
		logLevel = log.WARN
	case "error":
		logLevel = log.ERROR
	}
	log.Init(logLevel)

	log.Info("MCP Gateway starting", map[string]interface{}{
		"host":              g.host,
		"port":              g.port,
		"mode":              "mixed (per-server)",
		"available_servers": len(g.config.Servers),
		"log_level":         g.config.Settings.LogLevel,
	})

	fmt.Printf("MCP Gateway starting on %s:%d\n", g.host, g.port)
	fmt.Printf("Mode: mixed (per-server: containers for Docker images, processes otherwise)\n")
	fmt.Printf("Available servers: %d\n", len(g.config.Servers))

	if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("Gateway server error", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("gateway server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the gateway
func (g *Gateway) Stop(ctx context.Context) error {
	g.cancel() // Cancel background tasks

	// Stop all server instances
	g.servers.StopAll()

	// Shutdown HTTP server
	if err := g.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown gateway: %w", err)
	}

	return nil
}

// handleMCP handles MCP protocol messages (stateless, persistent servers)
func (g *Gateway) handleMCP(w http.ResponseWriter, r *http.Request) {
	log.Debug("HTTP request received", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	if r.Method != http.MethodPost {
		log.Warn("Invalid HTTP method", map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract server name from path (e.g., /mcp/filesystem) or header
	serverName := ""
	if len(r.URL.Path) > 5 && r.URL.Path[:5] == "/mcp/" {
		// Path-based routing: /mcp/{serverName}[/additional/path]
		// Extract only the first segment after /mcp/ as the server name
		pathAfterMcp := r.URL.Path[5:]
		// Find the next slash to isolate the server name
		if idx := len(pathAfterMcp); idx > 0 {
			// Check if there's another slash (for http_path suffixes)
			for i, c := range pathAfterMcp {
				if c == '/' {
					idx = i
					break
				}
			}
			serverName = pathAfterMcp[:idx]
		}
	} else {
		// Legacy: X-MCP-Server header
		serverName = r.Header.Get("X-MCP-Server")
	}

	if serverName == "" {
		log.Warn("Server name missing from request", map[string]interface{}{
			"path": r.URL.Path,
		})
		http.Error(w, "Server name required (use /mcp/{serverName} or X-MCP-Server header)", http.StatusBadRequest)
		return
	}

	log.Info("MCP request received", map[string]interface{}{
		"server_name": serverName,
		"path":        r.URL.Path,
	})

	// Get or create persistent server instance
	instance, err := g.servers.GetOrCreate(serverName, g)
	if err != nil {
		log.Error("Failed to get server instance", map[string]interface{}{
			"server_name": serverName,
			"error":       err.Error(),
		})
		http.Error(w, fmt.Sprintf("Failed to get server: %v", err), http.StatusInternalServerError)
		return
	}

	// Forward message to server
	response, err := instance.SendMessage(r.Context(), r.Body)
	if err != nil {
		log.Error("Failed to send message to server", map[string]interface{}{
			"server_name": serverName,
			"error":       err.Error(),
		})
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}

	log.Info("MCP request completed successfully", map[string]interface{}{
		"server_name":    serverName,
		"response_bytes": len(response),
	})

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// handleHealth returns gateway health status
func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	activeServers := g.servers.Count()
	g.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","active_servers":%d}`, activeServers)
}

// handleServers returns available MCP servers
func (g *Gateway) handleServers(w http.ResponseWriter, r *http.Request) {
	servers := g.config.GetEnabledServers()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("["))
	for i, server := range servers {
		if i > 0 {
			w.Write([]byte(","))
		}
		fmt.Fprintf(w, `{"name":"%s","description":"%s","image":"%s"}`,
			server.Name, server.Description, server.Docker.Image)
	}
	w.Write([]byte("]"))
}
