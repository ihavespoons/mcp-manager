package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/ihavespoons/mcp-manager/internal/claudecode"
	"github.com/ihavespoons/mcp-manager/internal/config"
	"github.com/ihavespoons/mcp-manager/internal/docker"
	"github.com/ihavespoons/mcp-manager/internal/gateway"
	"github.com/ihavespoons/mcp-manager/internal/mcpserver"
	"github.com/ihavespoons/mcp-manager/internal/server"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	configPath string
	verbose    bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "mcp-manager",
	Short: "Manage MCP servers in Docker containers",
	Long: `mcp-manager is a CLI tool that allows you to define and manage MCP servers
within Docker containers. It ensures your MCP servers are running and registered
with your AI agents (starting with Claude Code).`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file (default: ./mcp-config.yaml or ~/.mcp-manager/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	logsCmd.Flags().BoolP("follow", "f", false, "follow log output")
	logsCmd.Flags().IntP("tail", "n", 100, "number of lines to show from the end of the logs")

	startCmd.Flags().BoolP("all", "a", false, "start all servers")
	stopCmd.Flags().BoolP("all", "a", false, "stop all servers")
	updateCmd.Flags().BoolP("all", "a", false, "update all server images")

	// Gateway command flags
	gatewayCmd.Flags().StringP("host", "H", "0.0.0.0", "gateway host address")
	gatewayCmd.Flags().IntP("port", "p", 52080, "gateway port")
	gatewayCmd.Flags().StringP("mode", "m", "process", "server mode: container or process")
}

// registerWithClaudeCode registers mcp-manager and its servers with Claude Code
func registerWithClaudeCode(cfg *config.Config, configPath string) error {
	// Get current working directory (project path)
	projectPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get absolute path to mcp-manager binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get absolute path to config file
	absConfigPath := configPath
	if absConfigPath != "" {
		absConfigPath, err = filepath.Abs(absConfigPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute config path: %w", err)
		}
	}

	// Load Claude Code configuration
	claudeConfig, err := claudecode.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load Claude Code config: %w", err)
	}

	// Get list of enabled servers for this sync
	enabledServers := cfg.GetEnabledServers()
	gatewayPort := cfg.Settings.GatewayPort
	if gatewayPort == 0 {
		gatewayPort = 52080 // Default port
	}

	// Build set of valid server names that should exist in Claude Code
	validServerNames := make(map[string]bool)
	validServerNames["mcp-manager"] = true
	for _, srv := range enabledServers {
		validServerNames[srv.Name] = true
	}

	// Clean up any servers that are no longer in our config
	// We identify mcp-manager managed servers by:
	// 1. Having type="http" and URL matching our gateway pattern
	// 2. Being named "mcp-manager" (the stdio server)
	existingServers := claudeConfig.ListServers(projectPath)
	gatewayURLPrefix := fmt.Sprintf("http://localhost:%d/mcp/", gatewayPort)

	for serverName, serverConfig := range existingServers {
		if !validServerNames[serverName] {
			// Check if this server was managed by mcp-manager
			isManagedServer := false

			// mcp-manager stdio server
			if serverName == "mcp-manager" {
				isManagedServer = true
			} else if serverConfig.Type == "http" &&
				serverConfig.URL != "" &&
				len(serverConfig.URL) >= len(gatewayURLPrefix) &&
				serverConfig.URL[:len(gatewayURLPrefix)] == gatewayURLPrefix {
				// HTTP servers using our gateway URL pattern
				isManagedServer = true
			}

			if isManagedServer {
				claudeConfig.UnregisterServer(projectPath, serverName)
			}
		}
	}

	// Create MCP server config for mcp-manager serve
	cmdArgs := []string{"serve"}
	if absConfigPath != "" {
		cmdArgs = append(cmdArgs, "--config", absConfigPath)
	}

	mcpServer := &claudecode.MCPServer{
		Type:    "stdio",
		Command: execPath,
		Args:    cmdArgs,
	}

	// Register mcp-manager as an MCP server
	claudeConfig.RegisterServer(projectPath, "mcp-manager", mcpServer)

	// Register each enabled server as an HTTP endpoint through the gateway
	for _, srv := range enabledServers {
		// Convert to Claude Code HTTP MCP server format
		httpServer := claudecode.ServerFromConfig(&srv, gatewayPort)
		claudeConfig.RegisterServer(projectPath, srv.Name, httpServer)
	}

	// Save Claude Code configuration
	if err := claudeConfig.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save Claude Code config: %w", err)
	}

	return nil
}

// cleanupStaleContainers removes any stale containers from previous runs
func cleanupStaleContainers(dockerClient *docker.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List all containers managed by mcp-manager
	containers, err := dockerClient.ListContainers(ctx)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to list containers for cleanup: %v\n", err)
		}
		return
	}

	// Remove any running or stopped containers
	for _, container := range containers {
		if verbose {
			fmt.Fprintf(os.Stderr, "Cleaning up stale container: %s (state: %s)\n", container.Name, container.State)
		}
		// Force remove (stops if running, then removes)
		if err := dockerClient.RemoveContainer(ctx, container.ID, true); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to remove container %s: %v\n", container.Name, err)
			}
		}
	}

	if verbose && len(containers) > 0 {
		fmt.Fprintf(os.Stderr, "Cleaned up %d stale container(s)\n", len(containers))
	}
}

var startCmd = &cobra.Command{
	Use:   "start [server-name]",
	Short: "Start MCP servers",
	Long:  "Start one or all MCP servers defined in the configuration file",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client
		dockerClient, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Check Docker connection
		if err := dockerClient.Ping(ctx); err != nil {
			return fmt.Errorf("Docker daemon not available: %w", err)
		}

		// Create server manager
		mgr := server.NewManager(dockerClient, cfg)

		// Determine which server to start
		serverName := ""
		if len(args) > 0 {
			serverName = args[0]
		}

		// Start servers
		if verbose {
			fmt.Printf("Starting MCP servers...\n")
		}

		if err := mgr.Start(ctx, serverName); err != nil {
			return err
		}

		if serverName == "" {
			fmt.Println("All enabled MCP servers started successfully")
		} else {
			fmt.Printf("MCP server '%s' started successfully\n", serverName)
		}

		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [server-name]",
	Short: "Stop MCP servers",
	Long:  "Stop one or all running MCP servers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client
		dockerClient, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Create server manager
		mgr := server.NewManager(dockerClient, cfg)

		// Determine which server to stop
		serverName := ""
		if len(args) > 0 {
			serverName = args[0]
		}

		// Stop servers
		if verbose {
			fmt.Printf("Stopping MCP servers...\n")
		}

		if err := mgr.Stop(ctx, serverName); err != nil {
			return err
		}

		if serverName == "" {
			fmt.Println("All MCP servers stopped successfully")
		} else {
			fmt.Printf("MCP server '%s' stopped successfully\n", serverName)
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [server-name]",
	Short: "Show status of MCP servers",
	Long:  "Display the status of one or all MCP servers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client
		dockerClient, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Create server manager
		mgr := server.NewManager(dockerClient, cfg)

		// Determine which server to check
		serverName := ""
		if len(args) > 0 {
			serverName = args[0]
		}

		// Get status
		statuses, err := mgr.Status(ctx, serverName)
		if err != nil {
			return err
		}

		// Print status
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tIMAGE\tSTATE\tRUNNING")

		for _, status := range statuses {
			state := "not created"
			if status.Exists {
				state = status.State
			}
			running := "no"
			if status.Running {
				running = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", status.Name, status.Image, state, running)
		}

		w.Flush()
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured MCP servers",
	Long:  "Display all MCP servers defined in the configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create server manager (no Docker client needed for list)
		mgr := server.NewManager(nil, cfg)

		// Get list of servers
		servers := mgr.List()

		if len(servers) == 0 {
			fmt.Println("No servers configured")
			return nil
		}

		// Print servers
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tENABLED\tIMAGE\tDESCRIPTION")

		for _, srv := range servers {
			enabled := "no"
			if srv.Enabled {
				enabled = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", srv.Name, enabled, srv.Image, srv.Description)
		}

		w.Flush()
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <server-name>",
	Short: "Show logs from an MCP server",
	Long:  "Display logs from a running MCP server container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		serverName := args[0]

		// Get flags
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client
		dockerClient, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Create server manager
		mgr := server.NewManager(dockerClient, cfg)

		// Get log options
		_, err = mgr.GetLogs(ctx, serverName, follow, tail)
		if err != nil {
			return err
		}

		// Get container logs
		reader, err := dockerClient.GetContainerLogs(ctx, serverName, docker.LogOptions{
			Follow: follow,
			Tail:   tail,
		})
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer reader.Close()

		// Stream logs to stdout
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return fmt.Errorf("failed to read logs: %w", err)
		}

		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update [server-name]",
	Short: "Update MCP server images",
	Long:  "Pull the latest Docker images for one or all MCP servers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client
		dockerClient, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer dockerClient.Close()

		// Determine which servers to update
		servers := cfg.GetEnabledServers()
		if len(args) > 0 {
			serverName := args[0]
			found := false
			for _, srv := range cfg.Servers {
				if srv.Name == serverName {
					servers = []config.Server{srv}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("server %s not found", serverName)
			}
		}

		// Pull images
		for _, srv := range servers {
			if verbose {
				fmt.Printf("Pulling image %s...\n", srv.Docker.Image)
			}

			if err := dockerClient.PullImage(ctx, srv.Docker.Image, "always"); err != nil {
				return fmt.Errorf("failed to pull image for server %s: %w", srv.Name, err)
			}

			fmt.Printf("Updated image for server '%s'\n", srv.Name)
		}

		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  "Validate the MCP manager configuration file for errors",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration (this validates it)
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Println("Configuration is valid")
		if verbose {
			fmt.Printf("  Version: %s\n", cfg.Version)
			fmt.Printf("  Data directory: %s\n", cfg.Settings.DataDir)
			fmt.Printf("  Network: %s\n", cfg.Settings.Network)
			fmt.Printf("  Servers: %d\n", len(cfg.Servers))
			fmt.Printf("  Enabled servers: %d\n", len(cfg.GetEnabledServers()))
		}

		return nil
	},
}

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start MCP gateway server",
	Long:  "Start the MCP gateway server that bridges HTTP clients to stdio MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		mode, _ := cmd.Flags().GetString("mode")

		// Deprecation warning for mode flag
		if mode != "process" {
			fmt.Fprintf(os.Stderr, "Note: --mode flag is deprecated. Gateway now automatically uses containers for servers with docker.image, and processes otherwise.\n")
		}

		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create Docker client (needed even for process mode for some operations)
		dockerClient, err := docker.NewClient()
		if err != nil {
			if mode == "container" {
				return fmt.Errorf("failed to create Docker client: %w", err)
			}
			// In process mode, warn but continue
			fmt.Printf("Warning: Docker client unavailable, using process mode only\n")
		}
		if dockerClient != nil {
			defer dockerClient.Close()
		}

		// Create gateway
		gwConfig := gateway.Config{
			Host: host,
			Port: port,
		}

		gw := gateway.NewGateway(cfg, dockerClient, gwConfig)

		// Handle shutdown gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Start gateway in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- gw.Start()
		}()

		// Wait for shutdown signal or error
		select {
		case err := <-errChan:
			return fmt.Errorf("gateway error: %w", err)
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)

			// Graceful shutdown with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := gw.Stop(ctx); err != nil {
				return fmt.Errorf("failed to stop gateway: %w", err)
			}

			fmt.Println("Gateway stopped successfully")
			return nil
		}
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run mcp-manager as an MCP server",
	Long:  "Run mcp-manager as an MCP server (stdio) that manages the gateway and other MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Auto-register with Claude Code (updates registration if config changed)
		if err := registerWithClaudeCode(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to auto-register with Claude Code: %v\n", err)
			// Continue anyway - registration failure shouldn't block startup
		}

		// Create Docker client (optional in process mode)
		dockerClient, err := docker.NewClient()
		if err != nil {
			// In process mode, warn but continue
			fmt.Fprintf(os.Stderr, "Warning: Docker client unavailable, using process mode only\n")
		}
		if dockerClient != nil {
			defer dockerClient.Close()
		}

		// Cleanup any stale containers from previous runs
		if dockerClient != nil {
			cleanupStaleContainers(dockerClient)
		}

		// Create gateway config from settings
		gwConfig := gateway.Config{
			Host: "127.0.0.1",
			Port: cfg.Settings.GatewayPort,
		}

		// Create gateway
		gw := gateway.NewGateway(cfg, dockerClient, gwConfig)

		// Start gateway in background
		gwErrChan := make(chan error, 1)
		go func() {
			fmt.Fprintf(os.Stderr, "Starting gateway on port %d...\n", gwConfig.Port)
			gwErrChan <- gw.Start()
		}()

		// Give gateway a moment to start
		time.Sleep(100 * time.Millisecond)

		// Create MCP server
		mcpSrv := mcpserver.NewServer(cfg, gw)

		// Handle graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Monitor for gateway errors
		go func() {
			if err := <-gwErrChan; err != nil {
				fmt.Fprintf(os.Stderr, "Gateway error: %v\n", err)
				cancel()
			}
		}()

		// Start MCP server (blocks on stdin)
		serverErrChan := make(chan error, 1)
		go func() {
			serverErrChan <- mcpSrv.Serve()
		}()

		// Wait for MCP server to finish, context cancellation, or signal
		select {
		case err := <-serverErrChan:
			// stdin closed or error
			if err != nil {
				fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			}

			// Shutdown gateway
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if gwErr := gw.Stop(shutdownCtx); gwErr != nil {
				fmt.Fprintf(os.Stderr, "Gateway shutdown error: %v\n", gwErr)
			}

			return err

		case sig := <-sigChan:
			// Received shutdown signal
			fmt.Fprintf(os.Stderr, "\nReceived signal %v, shutting down...\n", sig)

			// Stop MCP server
			mcpSrv.Stop()

			// Shutdown gateway
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if gwErr := gw.Stop(shutdownCtx); gwErr != nil {
				fmt.Fprintf(os.Stderr, "Gateway shutdown error: %v\n", gwErr)
			}

			fmt.Fprintf(os.Stderr, "Shutdown complete\n")
			return nil

		case <-ctx.Done():
			// Gateway error or other cancellation
			mcpSrv.Stop()

			// Shutdown gateway
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if gwErr := gw.Stop(shutdownCtx); gwErr != nil {
				fmt.Fprintf(os.Stderr, "Gateway shutdown error: %v\n", gwErr)
			}

			return fmt.Errorf("server stopped due to context cancellation")
		}
	},
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register mcp-manager with Claude Code",
	Long:  "Register mcp-manager as an MCP server (stdio) with Claude Code for the current project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load mcp-manager configuration
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Register with Claude Code
		if err := registerWithClaudeCode(cfg, configPath); err != nil {
			return err
		}

		// Print summary
		enabledServers := cfg.GetEnabledServers()
		gatewayPort := cfg.Settings.GatewayPort
		if gatewayPort == 0 {
			gatewayPort = 52080 // Default port
		}

		fmt.Println("Successfully registered mcp-manager with Claude Code")
		fmt.Printf("\nRegistered %d enabled MCP servers:\n", len(enabledServers))
		for _, srv := range enabledServers {
			fmt.Printf("  • %s (http://localhost:%d/mcp/%s)\n", srv.Name, gatewayPort, srv.Name)
		}

		if len(enabledServers) < len(cfg.Servers) {
			disabledCount := len(cfg.Servers) - len(enabledServers)
			fmt.Printf("\n%d server(s) are disabled and were not registered\n", disabledCount)
		}

		fmt.Println("\nRestart Claude Code to apply changes.")
		fmt.Println("\nUse the following tools in Claude Code to manage servers:")
		fmt.Println("  • list_servers - List all configured MCP servers")
		fmt.Println("  • server_status - Get server status")
		fmt.Println("  • gateway_status - Get gateway status")

		return nil
	},
}

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Unregister mcp-manager from Claude Code",
	Long:  "Remove mcp-manager as an MCP server from Claude Code for the current project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load mcp-manager configuration to find all servers
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Get current working directory (project path)
		projectPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Load Claude Code configuration
		claudeConfig, err := claudecode.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load Claude Code config: %w", err)
		}

		// Unregister mcp-manager
		mcpManagerRemoved := claudeConfig.UnregisterServer(projectPath, "mcp-manager")

		// Unregister all MCP servers from config
		serversRemoved := 0
		for _, srv := range cfg.Servers {
			if claudeConfig.UnregisterServer(projectPath, srv.Name) {
				serversRemoved++
				if verbose {
					fmt.Printf("Unregistering server %s\n", srv.Name)
				}
			}
		}

		if !mcpManagerRemoved && serversRemoved == 0 {
			fmt.Println("mcp-manager was not registered for this project")
			return nil
		}

		if verbose {
			fmt.Printf("Unregistering mcp-manager from project %s\n", projectPath)
		}

		// Save Claude Code configuration
		if err := claudeConfig.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save Claude Code config: %w", err)
		}

		fmt.Println("Successfully unregistered mcp-manager from Claude Code")
		if serversRemoved > 0 {
			fmt.Printf("Also unregistered %d MCP server(s)\n", serversRemoved)
		}
		fmt.Println("\nRestart Claude Code to apply changes.")

		return nil
	},
}
