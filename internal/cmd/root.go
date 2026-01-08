package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	"github.com/githubnext/gh-aw-mcpg/internal/server"
	"github.com/spf13/cobra"
)

var (
	configFile  string
	configStdin bool
	listenAddr  string
	routedMode  bool
	unifiedMode bool
	envFile     string
	enableDIFC  bool
	debugLog    = logger.New("cmd:root")
	version     = "dev" // Default version, overridden by SetVersion
)

var rootCmd = &cobra.Command{
	Use:     "awmg",
	Short:   "MCPG MCP proxy server",
	Version: version,
	Long: `MCPG is a proxy server for Model Context Protocol (MCP) servers.
It provides routing, aggregation, and management of multiple MCP backend servers.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config.toml", "Path to config file")
	rootCmd.Flags().BoolVar(&configStdin, "config-stdin", false, "Read MCP server configuration from stdin (JSON format). When enabled, overrides --config")
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", "127.0.0.1:3000", "HTTP server listen address")
	rootCmd.Flags().BoolVar(&routedMode, "routed", false, "Run in routed mode (each backend at /mcp/<server>)")
	rootCmd.Flags().BoolVar(&unifiedMode, "unified", false, "Run in unified mode (all backends at /mcp)")
	rootCmd.Flags().StringVar(&envFile, "env", "", "Path to .env file to load environment variables")
	rootCmd.Flags().BoolVar(&enableDIFC, "enable-difc", false, "Enable DIFC enforcement and session requirement (requires sys___init call before tool access)")

	// Mark mutually exclusive flags
	rootCmd.MarkFlagsMutuallyExclusive("routed", "unified")

	// Add completion command
	rootCmd.AddCommand(newCompletionCmd())
}

func run(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	debugLog.Printf("Starting MCPG with config: %s, listen: %s", configFile, listenAddr)

	// Load .env file if specified
	if envFile != "" {
		debugLog.Printf("Loading environment from file: %s", envFile)
		if err := loadEnvFile(envFile); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if configStdin {
		log.Println("Reading configuration from stdin...")
		cfg, err = config.LoadFromStdin()
	} else {
		log.Printf("Reading configuration from %s...", configFile)
		cfg, err = config.LoadFromFile(configFile)
	}

	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	debugLog.Printf("Configuration loaded with %d servers", len(cfg.Servers))
	log.Printf("Loaded %d MCP server(s)", len(cfg.Servers))

	// Apply command-line flags to config
	cfg.EnableDIFC = enableDIFC
	if enableDIFC {
		log.Println("DIFC enforcement and session requirement enabled")
	} else {
		log.Println("DIFC enforcement disabled (sessions auto-created for standard MCP client compatibility)")
	}

	// Determine mode (default to unified if neither flag is set)
	mode := "unified"
	if routedMode {
		mode = "routed"
	}

	debugLog.Printf("Server mode: %s, DIFC enabled: %v", mode, cfg.EnableDIFC)

	// Create unified MCP server (backend for both modes)
	unifiedServer, err := server.NewUnified(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create unified server: %w", err)
	}
	defer unifiedServer.Close()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
		unifiedServer.Close()
		os.Exit(0)
	}()

	// Create HTTP server based on mode
	var httpServer *http.Server
	if mode == "routed" {
		log.Printf("Starting MCPG in ROUTED mode on %s", listenAddr)
		log.Printf("Routes: /mcp/<server> where <server> is one of: %v", unifiedServer.GetServerIDs())

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}

		httpServer = server.CreateHTTPServerForRoutedMode(listenAddr, unifiedServer, apiKey)
	} else {
		log.Printf("Starting MCPG in UNIFIED mode on %s", listenAddr)
		log.Printf("Endpoint: /mcp")

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}

		httpServer = server.CreateHTTPServerForMCP(listenAddr, unifiedServer, apiKey)
	}
	// Start HTTP server in background
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	// Write gateway configuration to stdout per spec section 5.4
	if err := writeGatewayConfigToStdout(cfg, listenAddr, mode); err != nil {
		log.Printf("Warning: failed to write gateway configuration to stdout: %v", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	return nil
}

// writeGatewayConfigToStdout writes the rewritten gateway configuration to stdout
// per MCP Gateway Specification Section 5.4
func writeGatewayConfigToStdout(cfg *config.Config, listenAddr, mode string) error {
	return writeGatewayConfig(cfg, listenAddr, mode, os.Stdout)
}

func writeGatewayConfig(cfg *config.Config, listenAddr, mode string, w io.Writer) error {
	// Parse listen address to extract host and port
	host, port := "127.0.0.1", "3000"
	if parts := strings.Split(listenAddr, ":"); len(parts) == 2 {
		if parts[0] != "" {
			host = parts[0]
		}
		port = parts[1]
	}

	// Determine domain (use host from listen address)
	domain := host

	// Build output configuration
	outputConfig := map[string]interface{}{
		"mcpServers": make(map[string]interface{}),
	}

	servers := outputConfig["mcpServers"].(map[string]interface{})

	for name := range cfg.Servers {
		serverConfig := map[string]interface{}{
			"type": "http",
		}

		if mode == "routed" {
			serverConfig["url"] = fmt.Sprintf("http://%s:%s/mcp/%s", domain, port, name)
		} else {
			// Unified mode - all servers use /mcp endpoint
			serverConfig["url"] = fmt.Sprintf("http://%s:%s/mcp", domain, port)
		}

		// Note: apiKey would be added to headers if implemented
		// serverConfig["headers"] = map[string]string{"Authorization": apiKey}

		servers[name] = serverConfig
	}

	// Write to output as single JSON document
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(outputConfig); err != nil {
		return fmt.Errorf("failed to encode configuration: %w", err)
	}

	// Flush stdout buffer if it's a file
	if f, ok := w.(*os.File); ok {
		if err := f.Sync(); err != nil {
			return fmt.Errorf("failed to flush stdout: %w", err)
		}
	}

	return nil
}

// loadEnvFile reads a .env file and sets environment variables
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	log.Printf("Loading environment from %s...", path)
	scanner := bufio.NewScanner(file)
	loadedVars := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Expand $VAR references in value
		value = os.ExpandEnv(value)

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}

		// Log loaded variable (hide sensitive values)
		displayValue := value
		if len(value) > 0 {
			displayValue = value[:min(10, len(value))] + "..."
		}
		log.Printf("  Loaded: %s=%s", key, displayValue)
		loadedVars++
	}

	log.Printf("Loaded %d environment variables from %s", loadedVars, path)

	return scanner.Err()
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// SetVersion sets the version string for the CLI
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}
