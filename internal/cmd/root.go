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
	validateEnv bool
	debugLog    = logger.New("cmd:root")
	version     = "dev" // Default version, overridden by SetVersion
)

var rootCmd = &cobra.Command{
	Use:     "awmg",
	Short:   "MCPG MCP proxy server",
	Version: version,
	Long: `MCPG is a proxy server for Model Context Protocol (MCP) servers.
It provides routing, aggregation, and management of multiple MCP backend servers.`,
	SilenceUsage: true, // Don't show help on runtime errors
	RunE:         run,
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
	rootCmd.Flags().BoolVar(&validateEnv, "validate-env", false, "Validate execution environment (Docker, env vars) before starting")

	// Mark mutually exclusive flags
	rootCmd.MarkFlagsMutuallyExclusive("routed", "unified")

	// Add completion command
	rootCmd.AddCommand(newCompletionCmd())
}

func run(cmd *cobra.Command, args []string) error {
	debugLog.Printf("=== run() function started ===")
	logger.LogInfo("startup", "Starting MCPG Gateway run function")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	debugLog.Printf("Context created, initializing file logger...")
	// Initialize file logger early
	if err := logger.InitFileLogger(logDir, "mcp-gateway.log"); err != nil {
		log.Printf("Warning: Failed to initialize file logger: %v", err)
		debugLog.Printf("File logger initialization failed: %v", err)
	} else {
		debugLog.Printf("File logger initialized successfully: dir=%s", logDir)
	}
	defer logger.CloseGlobalLogger()

	logger.LogInfo("startup", "MCPG Gateway version: %s", version)
	logger.LogInfo("startup", "Starting MCPG with config: %s, listen: %s, log-dir: %s", configFile, listenAddr, logDir)
	debugLog.Printf("Starting MCPG with config: %s, listen: %s", configFile, listenAddr)

	// Load .env file if specified
	if envFile != "" {
		debugLog.Printf("Loading environment from file: %s", envFile)
		logger.LogInfo("startup", "Loading environment file: %s", envFile)
		if err := loadEnvFile(envFile); err != nil {
			debugLog.Printf("Failed to load .env file: %v", err)
			logger.LogError("startup", "Failed to load .env file: %v", err)
			return fmt.Errorf("failed to load .env file: %w", err)
		}
		debugLog.Printf("Environment file loaded successfully")
		logger.LogInfo("startup", "Environment file loaded successfully")
	}

	// Validate execution environment if requested
	if validateEnv {
		debugLog.Printf("Validating execution environment...")
		logger.LogInfo("startup", "Starting environment validation")
		result := config.ValidateExecutionEnvironment()
		if !result.IsValid() {
			logger.LogError("startup", "Environment validation failed: %s", result.Error())
			debugLog.Printf("Environment validation failed: %s", result.Error())
			return fmt.Errorf("environment validation failed: %s", result.Error())
		}
		logger.LogInfo("startup", "Environment validation passed")
		log.Println("Environment validation passed")
		debugLog.Printf("Environment validation passed")
	}

	// Load configuration
	debugLog.Printf("Loading configuration...")
	logger.LogInfo("startup", "Loading configuration: configStdin=%v, configFile=%s", configStdin, configFile)
	var cfg *config.Config
	var err error

	if configStdin {
		log.Println("Reading configuration from stdin...")
		debugLog.Printf("Loading config from stdin...")
		cfg, err = config.LoadFromStdin()
	} else {
		log.Printf("Reading configuration from %s...", configFile)
		debugLog.Printf("Loading config from file: %s", configFile)
		cfg, err = config.LoadFromFile(configFile)
	}

	if err != nil {
		debugLog.Printf("Failed to load config: %v", err)
		logger.LogError("startup", "Failed to load configuration: %v", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	debugLog.Printf("Configuration loaded with %d servers", len(cfg.Servers))
	log.Printf("Loaded %d MCP server(s)", len(cfg.Servers))
	logger.LogInfo("startup", "Configuration loaded successfully: servers=%d", len(cfg.Servers))

	// Apply command-line flags to config
	debugLog.Printf("Applying command-line flags to config: enableDIFC=%v", enableDIFC)
	cfg.EnableDIFC = enableDIFC
	if enableDIFC {
		log.Println("DIFC enforcement and session requirement enabled")
		logger.LogInfo("startup", "DIFC enforcement enabled")
	} else {
		log.Println("DIFC enforcement disabled (sessions auto-created for standard MCP client compatibility)")
		logger.LogInfo("startup", "DIFC enforcement disabled (standard MCP client mode)")
	}

	// Determine mode (default to unified if neither flag is set)
	mode := "unified"
	if routedMode {
		mode = "routed"
	}

	debugLog.Printf("Server mode: %s, DIFC enabled: %v", mode, cfg.EnableDIFC)
	logger.LogInfo("startup", "Creating unified MCP server: mode=%s, DIFC=%v", mode, cfg.EnableDIFC)

	// Create unified MCP server (backend for both modes)
	debugLog.Printf("Calling server.NewUnified()...")
	unifiedServer, err := server.NewUnified(ctx, cfg)
	if err != nil {
		debugLog.Printf("Failed to create unified server: %v", err)
		logger.LogError("startup", "Failed to create unified server: %v", err)
		return fmt.Errorf("failed to create unified server: %w", err)
	}
	debugLog.Printf("Unified server created successfully")
	logger.LogInfo("startup", "Unified server created successfully")
	defer unifiedServer.Close()

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
	debugLog.Printf("Creating HTTP server for mode: %s", mode)
	var httpServer *http.Server
	if mode == "routed" {
		log.Printf("Starting MCPG in ROUTED mode on %s", listenAddr)
		log.Printf("Routes: /mcp/<server> where <server> is one of: %v", unifiedServer.GetServerIDs())
		logger.LogInfo("startup", "Creating HTTP server in ROUTED mode: addr=%s, backends=%v", listenAddr, unifiedServer.GetServerIDs())

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}
		debugLog.Printf("Creating routed mode HTTP server with apiKey present: %v", apiKey != "")

		httpServer = server.CreateHTTPServerForRoutedMode(listenAddr, unifiedServer, apiKey)
		debugLog.Printf("Routed mode HTTP server created")
	} else {
		log.Printf("Starting MCPG in UNIFIED mode on %s", listenAddr)
		log.Printf("Endpoint: /mcp")
		logger.LogInfo("startup", "Creating HTTP server in UNIFIED mode: addr=%s", listenAddr)

		// Extract API key from gateway config (spec 7.1)
		apiKey := ""
		if cfg.Gateway != nil {
			apiKey = cfg.Gateway.APIKey
		}
		debugLog.Printf("Creating unified mode HTTP server with apiKey present: %v", apiKey != "")

		httpServer = server.CreateHTTPServerForMCP(listenAddr, unifiedServer, apiKey)
		debugLog.Printf("Unified mode HTTP server created")
	}

	debugLog.Printf("HTTP server created, starting in background goroutine...")
	logger.LogInfo("startup", "Starting HTTP server in background on %s", listenAddr)

	// Start HTTP server in background
	go func() {
		debugLog.Printf("HTTP server goroutine started, calling ListenAndServe()...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			logger.LogError("startup", "HTTP server error: %v", err)
			debugLog.Printf("HTTP server failed with error: %v", err)
			cancel()
		}
		debugLog.Printf("HTTP server ListenAndServe() returned")
	}()

	debugLog.Printf("HTTP server goroutine launched, writing gateway configuration to stdout...")
	logger.LogInfo("startup", "HTTP server background task started, preparing to write configuration")

	// Write gateway configuration to stdout per spec section 5.4
	if err := writeGatewayConfigToStdout(cfg, listenAddr, mode); err != nil {
		log.Printf("Warning: failed to write gateway configuration to stdout: %v", err)
		logger.LogError("startup", "Failed to write gateway configuration to stdout: %v", err)
		debugLog.Printf("writeGatewayConfigToStdout failed: %v", err)
	} else {
		debugLog.Printf("writeGatewayConfigToStdout completed successfully")
		logger.LogInfo("startup", "Gateway configuration successfully written to stdout")
	}

	debugLog.Printf("Entering wait-for-shutdown phase...")
	logger.LogInfo("startup", "Gateway initialization complete, waiting for shutdown signal")
	log.Println("Gateway is ready and waiting for connections...")

	// Wait for shutdown signal
	<-ctx.Done()

	debugLog.Printf("Shutdown signal received, run() returning...")
	logger.LogInfo("shutdown", "Shutdown signal received, exiting run() function")
	return nil
}

// writeGatewayConfigToStdout writes the rewritten gateway configuration to stdout
// per MCP Gateway Specification Section 5.4
func writeGatewayConfigToStdout(cfg *config.Config, listenAddr, mode string) error {
	return writeGatewayConfig(cfg, listenAddr, mode, os.Stdout)
}

func writeGatewayConfig(cfg *config.Config, listenAddr, mode string, w io.Writer) error {
	debugLog.Printf("Starting writeGatewayConfig: listenAddr=%s, mode=%s, servers=%d", listenAddr, mode, len(cfg.Servers))
	logger.LogInfo("startup", "Writing gateway configuration to stdout: mode=%s, servers=%d", mode, len(cfg.Servers))

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
		debugLog.Printf("Parsed listen address: host=%s, port=%s", host, port)
	} else {
		debugLog.Printf("Failed to parse listen address, using defaults: host=%s, port=%s, error=%v", host, port, err)
	}

	// Determine domain (use host from listen address)
	domain := host

	// Build output configuration
	debugLog.Printf("Building output configuration for %d servers", len(cfg.Servers))
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
			debugLog.Printf("Added routed server config: name=%s, url=%s", name, serverConfig["url"])
		} else {
			// Unified mode - all servers use /mcp endpoint
			serverConfig["url"] = fmt.Sprintf("http://%s:%s/mcp", domain, port)
			debugLog.Printf("Added unified server config: name=%s, url=%s", name, serverConfig["url"])
		}

		// Note: apiKey would be added to headers if implemented
		// serverConfig["headers"] = map[string]string{"Authorization": apiKey}

		servers[name] = serverConfig
	}

	// Write to output as single JSON document
	debugLog.Printf("Encoding configuration to JSON...")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(outputConfig); err != nil {
		debugLog.Printf("Failed to encode configuration: %v", err)
		logger.LogError("startup", "Failed to encode gateway configuration: %v", err)
		return fmt.Errorf("failed to encode configuration: %w", err)
	}
	debugLog.Printf("Configuration encoded and written successfully")
	logger.LogInfo("startup", "Gateway configuration written to stdout successfully")

	// Flush stdout buffer if it's a regular file
	// Note: Sync() fails on pipes and character devices like /dev/stdout,
	// which is expected behavior. We only sync regular files.
	debugLog.Printf("Checking if output is a regular file for syncing...")
	if f, ok := w.(*os.File); ok {
		debugLog.Printf("Output is a file descriptor, checking mode...")
		if info, err := f.Stat(); err == nil && info.Mode().IsRegular() {
			debugLog.Printf("Output is a regular file, calling Sync()...")
			if err := f.Sync(); err != nil {
				// Log warning but don't fail - sync is best-effort
				debugLog.Printf("Warning: failed to sync file: %v", err)
				logger.LogWarn("startup", "Failed to sync stdout file: %v", err)
			} else {
				debugLog.Printf("Sync() completed successfully")
				logger.LogDebug("startup", "Successfully synced gateway configuration to stdout")
			}
		} else if err != nil {
			debugLog.Printf("Failed to stat file: %v", err)
			logger.LogDebug("startup", "Could not stat output file: %v", err)
		} else {
			debugLog.Printf("Output is not a regular file (mode: %v), skipping Sync()", info.Mode())
			logger.LogDebug("startup", "Output is not a regular file, skipping sync")
		}
	} else {
		debugLog.Printf("Output writer is not a file, skipping Sync()")
		logger.LogDebug("startup", "Output writer is not a file, skipping sync")
	}

	debugLog.Printf("writeGatewayConfig completed successfully")
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
	config.SetVersion(v)
}
