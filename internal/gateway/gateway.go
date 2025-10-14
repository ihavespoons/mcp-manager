package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bengittins/mcp-manager/internal/config"
	"github.com/bengittins/mcp-manager/internal/docker"
)

// Gateway manages MCP client connections and server lifecycle
type Gateway struct {
	config   *config.Config
	docker   *docker.Client
	host     string
	port     int
	server   *http.Server
	sessions *SessionManager
	mode     string // "container" or "process"
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// Config contains gateway configuration
type Config struct {
	Host           string
	Port           int
	Mode           string // "container" or "process"
	SessionTimeout time.Duration
}

// NewGateway creates a new MCP gateway
func NewGateway(cfg *config.Config, dockerClient *docker.Client, gwConfig Config) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())

	gw := &Gateway{
		config:   cfg,
		docker:   dockerClient,
		host:     gwConfig.Host,
		port:     gwConfig.Port,
		mode:     gwConfig.Mode,
		sessions: NewSessionManager(gwConfig.SessionTimeout),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/", gw.handleMCP)      // Path-based routing: /mcp/{serverName}
	mux.HandleFunc("/mcp", gw.handleMCP)       // Legacy: /mcp with X-MCP-Server header
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
	go g.sessions.CleanupLoop(g.ctx)

	fmt.Printf("MCP Gateway starting on %s:%d\n", g.host, g.port)
	fmt.Printf("Mode: %s\n", g.mode)
	fmt.Printf("Available servers: %d\n", len(g.config.Servers))

	if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("gateway server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the gateway
func (g *Gateway) Stop(ctx context.Context) error {
	g.cancel() // Cancel background tasks

	// Stop all sessions
	g.sessions.StopAll()

	// Shutdown HTTP server
	if err := g.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown gateway: %w", err)
	}

	return nil
}

// handleMCP handles MCP protocol messages
func (g *Gateway) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get or create session
	sessionID := r.Header.Get("X-Session-ID")

	// Extract server name from path (e.g., /mcp/filesystem) or header
	serverName := ""
	if len(r.URL.Path) > 5 && r.URL.Path[:5] == "/mcp/" {
		// Path-based routing: /mcp/{serverName}
		serverName = r.URL.Path[5:]
	} else {
		// Legacy: X-MCP-Server header
		serverName = r.Header.Get("X-MCP-Server")
	}

	if serverName == "" {
		http.Error(w, "Server name required (use /mcp/{serverName} or X-MCP-Server header)", http.StatusBadRequest)
		return
	}

	var session *Session
	var err error

	if sessionID == "" {
		// Create new session
		session, err = g.sessions.Create(serverName, g)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
			return
		}
		sessionID = session.ID
		w.Header().Set("X-Session-ID", sessionID)
	} else {
		// Get existing session
		session = g.sessions.Get(sessionID)
		if session == nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
	}

	// Forward message to server
	response, err := session.SendMessage(r.Context(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// handleHealth returns gateway health status
func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	activeSessions := g.sessions.Count()
	g.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","active_sessions":%d}`, activeSessions)
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
