package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ihavespoons/mcp-manager/internal/config"
)

// MCPServer represents an MCP server configuration for Claude Code
type MCPServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// ProjectConfig represents a project's configuration in Claude Code
type ProjectConfig struct {
	AllowedTools                            []string              `json:"allowedTools"`
	History                                 []interface{}         `json:"history"`
	MCPContextUris                          []string              `json:"mcpContextUris"`
	MCPServers                              map[string]*MCPServer `json:"mcpServers"`
	EnabledMcpjsonServers                   []string              `json:"enabledMcpjsonServers"`
	DisabledMcpjsonServers                  []string              `json:"disabledMcpjsonServers"`
	HasTrustDialogAccepted                  bool                  `json:"hasTrustDialogAccepted"`
	IgnorePatterns                          []string              `json:"ignorePatterns,omitempty"`
	ProjectOnboardingSeenCount              int                   `json:"projectOnboardingSeenCount"`
	HasClaudeMdExternalIncludesApproved     bool                  `json:"hasClaudeMdExternalIncludesApproved"`
	HasClaudeMdExternalIncludesWarningShown bool                  `json:"hasClaudeMdExternalIncludesWarningShown"`
}

// ClaudeConfig represents the entire Claude Code configuration
type ClaudeConfig struct {
	Projects map[string]*ProjectConfig `json:"projects"`
	// Include other fields as needed, but we'll preserve them when reading/writing
	rawConfig map[string]interface{}
}

// GetConfigPath returns the path to the Claude Code configuration file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".claude.json"), nil
}

// LoadConfig loads the Claude Code configuration
func LoadConfig() (*ClaudeConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config doesn't exist, create new one
			return &ClaudeConfig{
				Projects:  make(map[string]*ProjectConfig),
				rawConfig: make(map[string]interface{}),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cc := &ClaudeConfig{
		Projects:  make(map[string]*ProjectConfig),
		rawConfig: rawConfig,
	}

	// Extract projects
	if projectsRaw, ok := rawConfig["projects"].(map[string]interface{}); ok {
		for path, projRaw := range projectsRaw {
			projData, _ := json.Marshal(projRaw)
			var proj ProjectConfig
			if err := json.Unmarshal(projData, &proj); err != nil {
				continue // Skip invalid projects
			}
			if proj.MCPServers == nil {
				proj.MCPServers = make(map[string]*MCPServer)
			}
			cc.Projects[path] = &proj
		}
	} else {
		// No projects yet
		rawConfig["projects"] = make(map[string]interface{})
	}

	return cc, nil
}

// SaveConfig saves the Claude Code configuration
func (cc *ClaudeConfig) SaveConfig() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Merge projects back into rawConfig
	projectsMap := make(map[string]interface{})
	for path, proj := range cc.Projects {
		projData, _ := json.Marshal(proj)
		var projRaw map[string]interface{}
		json.Unmarshal(projData, &projRaw)
		projectsMap[path] = projRaw
	}
	cc.rawConfig["projects"] = projectsMap

	data, err := json.MarshalIndent(cc.rawConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// RegisterServer registers an MCP server for a specific project
func (cc *ClaudeConfig) RegisterServer(projectPath, serverName string, server *MCPServer) {
	if _, exists := cc.Projects[projectPath]; !exists {
		cc.Projects[projectPath] = &ProjectConfig{
			AllowedTools:                            []string{},
			History:                                 []interface{}{},
			MCPContextUris:                          []string{},
			MCPServers:                              make(map[string]*MCPServer),
			EnabledMcpjsonServers:                   []string{},
			DisabledMcpjsonServers:                  []string{},
			HasTrustDialogAccepted:                  false,
			ProjectOnboardingSeenCount:              0,
			HasClaudeMdExternalIncludesApproved:     false,
			HasClaudeMdExternalIncludesWarningShown: false,
		}
	}

	if cc.Projects[projectPath].MCPServers == nil {
		cc.Projects[projectPath].MCPServers = make(map[string]*MCPServer)
	}

	cc.Projects[projectPath].MCPServers[serverName] = server
}

// UnregisterServer removes an MCP server from a project
func (cc *ClaudeConfig) UnregisterServer(projectPath, serverName string) bool {
	if proj, exists := cc.Projects[projectPath]; exists {
		if _, serverExists := proj.MCPServers[serverName]; serverExists {
			delete(proj.MCPServers, serverName)
			return true
		}
	}
	return false
}

// ListServers lists all MCP servers for a project
func (cc *ClaudeConfig) ListServers(projectPath string) map[string]*MCPServer {
	if proj, exists := cc.Projects[projectPath]; exists {
		return proj.MCPServers
	}
	return make(map[string]*MCPServer)
}

// ServerFromConfig converts an mcp-manager server config to Claude Code format
func ServerFromConfig(srv *config.Server, gatewayPort int) *MCPServer {
	// All servers are registered as HTTP through the gateway
	// The gateway handles stdio communication internally and appends http_path when needed
	url := fmt.Sprintf("http://localhost:%d/mcp/%s", gatewayPort, srv.Name)

	return &MCPServer{
		Type: "http",
		URL:  url,
	}
}
