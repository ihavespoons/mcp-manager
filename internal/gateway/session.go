package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bengittins/mcp-manager/internal/config"
)

// Session represents an active MCP client session
type Session struct {
	ID            string
	ServerName    string
	ContainerID   string
	ProcessID     int
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	messageWriter *MessageWriter
	messageReader *MessageReader
	Created       time.Time
	LastActivity  time.Time
	mu            sync.RWMutex
}

// SessionManager manages active sessions
type SessionManager struct {
	sessions map[string]*Session
	timeout  time.Duration
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(timeout time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}
}

// Create creates a new session and spawns the MCP server
func (sm *SessionManager) Create(serverName string, gateway *Gateway) (*Session, error) {
	sessionID := generateSessionID()

	session := &Session{
		ID:           sessionID,
		ServerName:   serverName,
		Created:      time.Now(),
		LastActivity: time.Now(),
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
		return nil, fmt.Errorf("server %s not found or not enabled", serverName)
	}

	// Spawn server based on mode
	var err error
	if gateway.mode == "container" {
		err = session.spawnContainer(context.Background(), gateway, *serverConfig)
	} else {
		err = session.spawnProcess(context.Background(), gateway, *serverConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to spawn server: %w", err)
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	return session, nil
}

// Get retrieves a session by ID
func (sm *SessionManager) Get(sessionID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[sessionID]
}

// Delete removes a session
func (sm *SessionManager) Delete(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// Count returns the number of active sessions
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// CleanupLoop periodically cleans up expired sessions
func (sm *SessionManager) CleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.cleanup()
		}
	}
}

// cleanup removes expired sessions
func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		session.mu.RLock()
		expired := now.Sub(session.LastActivity) > sm.timeout
		session.mu.RUnlock()

		if expired {
			session.Stop()
			delete(sm.sessions, id)
		}
	}
}

// StopAll stops all active sessions
func (sm *SessionManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		session.Stop()
	}
	sm.sessions = make(map[string]*Session)
}

// SendMessage sends a message to the MCP server and returns the response
func (s *Session) SendMessage(ctx context.Context, body io.Reader) ([]byte, error) {
	s.mu.Lock()
	s.LastActivity = time.Now()
	s.mu.Unlock()

	// Read the request body
	requestData, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Parse as JSON-RPC message
	msg, err := ParseRequest(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	// Write message to server stdin with newline delimiter
	if err := s.messageWriter.WriteMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to write to server: %w", err)
	}

	// Read response from server stdout
	response, err := s.messageReader.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read from server: %w", err)
	}

	// Serialize response back to JSON (without newline for HTTP response)
	responseData, err := SerializeMessage(response)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize response: %w", err)
	}

	return responseData, nil
}

// Stop stops the session and cleans up resources
func (s *Session) Stop() {
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
