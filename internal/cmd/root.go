package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
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

// Default values for command-line flags.
const (
	defaultConfigFile  = "config.toml"
	defaultConfigStdin = false
	// DefaultListenIPv4 is the default interface used by the HTTP server.
	DefaultListenIPv4 = "127.0.0.1"
	// DefaultListenPort is the default port used by the HTTP server.
	DefaultListenPort  = "3000"
	defaultListenAddr  = DefaultListenIPv4 + ":" + DefaultListenPort
	defaultRoutedMode  = false
	defaultUnifiedMode = false
	defaultEnvFile     = ""
	defaultEnableDIFC  = false
	defaultLogDir      = "/tmp/gh-aw/sandbox/mcp"
)

var (
	configFile  string
	configStdin bool
	listenAddr  string
	routedMode  bool
	unifiedMode bool
	envFile     string
	enableDIFC  bool
	logDir      string
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
	rootCmd.Flags().StringVarP(&configFile, "config", "c", defaultConfigFile, "Path to config file")
	rootCmd.Flags().BoolVar(&configStdin, "config-stdin", defaultConfigStdin, "Read MCP server configuration from stdin (JSON format). When enabled, overrides --config")
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", defaultListenAddr, "HTTP server listen address")
	rootCmd.Flags().BoolVar(&routedMode, "routed", defaultRoutedMode, "Run in routed mode (each backend at /mcp/<server>)")
	rootCmd.Flags().BoolVar(&unifiedMode, "unified", defaultUnifiedMode, "Run in unified mode (all backends at /mcp)")
	rootCmd.Flags().StringVar(&envFile, "env", defaultEnvFile, "Path to .env file to load environment variables")
	rootCmd.Flags().BoolVar(&enableDIFC, "enable-difc", defaultEnableDIFC, "Enable DIFC enforcement and session requirement (requires sys___init call before tool access)")
	rootCmd.Flags().StringVar(&logDir, "log-dir", defaultLogDir, "Directory for log files (falls back to stdout if directory cannot be created)")

	// Mark mutually exclusive flags
	rootCmd.MarkFlagsMutuallyExclusive("routed", "unified")

	// Add completion command
	rootCmd.AddCommand(newCompletionCmd())
}

func run(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize file logger early
	debugLog.Printf("Initializing file logger: logDir=%s", logDir)
	if err := logger.InitFileLogger(logDir, "mcp-gateway.log"); err != nil {
		log.Printf("Warning: Failed to initialize file logger: %v", err)
	}
	defer logger.CloseGlobalLogger()

	logger.LogInfo("startup", "Starting MCPG with config: %s, listen: %s, log-dir: %s", configFile, listenAddr, logDir)
	debugLog.Printf("Starting MCPG with config: %s, listen: %s, routed: %v, unified: %v, enableDIFC: %v",
		configFile, listenAddr, routedMode, unifiedMode, enableDIFC)

	// Load .env file if specified
	if envFile != "" {
		debugLog.Printf("Loading environment from file: %s", envFile)
		logger.LogInfo("startup", "Loading environment variables from file: %s", envFile)
		if err := loadEnvFile(envFile); err != nil {
			logger.LogError("startup", "Failed to load .env file: %s, error: %v", envFile, err)
			return fmt.Errorf("failed to load .env file: %w", err)
		}
		logger.LogInfo("startup", "Environment variables loaded successfully from: %s", envFile)
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if configStdin {
		log.Println("Reading configuration from stdin...")
		logger.LogInfo("startup", "Loading configuration from stdin (JSON format)")
		debugLog.Print("Reading configuration from stdin in JSON format")
		cfg, err = config.LoadFromStdin()
	} else {
		log.Printf("Reading configuration from %s...", configFile)
		logger.LogInfo("startup", "Loading configuration from file: %s", configFile)
		debugLog.Printf("Reading configuration from file: %s", configFile)
		cfg, err = config.LoadFromFile(configFile)
	}

	if err != nil {
		logger.LogError("startup", "Configuration loading failed: %v", err)
		debugLog.Printf("Configuration loading error: %v", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	debugLog.Printf("Configuration loaded with %d servers", len(cfg.Servers))
	log.Printf("Loaded %d MCP server(s)", len(cfg.Servers))
	logger.LogInfo("startup", "Configuration loaded successfully: %d servers configured", len(cfg.Servers))

	// Apply command-line flags to config
	cfg.EnableDIFC = enableDIFC
	if enableDIFC {
		log.Println("DIFC enforcement and session requirement enabled")
		logger.LogInfo("startup", "DIFC enforcement enabled")
	} else {
		log.Println("DIFC enforcement disabled (sessions auto-created for standard MCP client compatibility)")
		logger.LogInfo("startup", "DIFC enforcement disabled (standard MCP client compatibility mode)")
	}

	// Determine mode (default to unified if neither flag is set)
	mode := "unified"
	if routedMode {
		mode = "routed"
	}

	debugLog.Printf("Server mode: %s, DIFC enabled: %v", mode, cfg.EnableDIFC)
	logger.LogInfo("startup", "Server mode: %s", mode)

	// Create unified MCP server (backend for both modes)
	debugLog.Printf("Creating unified MCP server")
	logger.LogInfo("startup", "Initializing unified MCP server")
	unifiedServer, err := server.NewUnified(ctx, cfg)
	if err != nil {
		logger.LogError("startup", "Failed to create unified server: %v", err)
		return fmt.Errorf("failed to create unified server: %w", err)
	}
	defer unifiedServer.Close()
	logger.LogInfo("startup", "Unified MCP server created successfully")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.LogInfo("shutdown", "Shutting down gateway...")
		log.Println("Shutting down...")
		cancel()
		unifiedServer.Close()
		logger.CloseGlobalLogger()
		os.Exit(0)
	}()

	// Create HTTP server based on mode
	var httpServer *http.Server
	if mode == "routed" {
		log.Printf("Starting MCPG in ROUTED mode on %s", listenAddr)
		log.Printf("Routes: /mcp/<server> where <server> is one of: %v", unifiedServer.GetServerIDs())
		logger.LogInfo("startup", "Creating HTTP server in ROUTED mode: %s", listenAddr)
		debugLog.Printf("ROUTED mode: endpoints for %v", unifiedServer.GetServerIDs())

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}

		httpServer = server.CreateHTTPServerForRoutedMode(listenAddr, unifiedServer, apiKey)
	} else {
		log.Printf("Starting MCPG in UNIFIED mode on %s", listenAddr)
		log.Printf("Endpoint: /mcp")
		logger.LogInfo("startup", "Creating HTTP server in UNIFIED mode: %s", listenAddr)
		debugLog.Printf("UNIFIED mode: single endpoint at /mcp")

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}

		httpServer = server.CreateHTTPServerForMCP(listenAddr, unifiedServer, apiKey)
	}
	// Start HTTP server in background
	logger.LogInfo("startup", "Starting HTTP server in background")
	go func() {
		debugLog.Printf("HTTP server listening on %s", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogError("startup", "HTTP server error: %v", err)
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	logger.LogInfo("startup", "Gateway initialization complete, ready to serve requests")

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
	// Use net.SplitHostPort which properly handles both IPv4 and IPv6 addresses
	host, port := DefaultListenIPv4, DefaultListenPort
	if h, p, err := net.SplitHostPort(listenAddr); err == nil {
		if h != "" {
			host = h
		}
		if p != "" {
			port = p
		}
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
