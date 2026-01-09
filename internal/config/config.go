package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var logConfig = logger.New("config:config")

// Config represents the MCPG configuration
type Config struct {
	Servers    map[string]*ServerConfig `toml:"servers"`
	EnableDIFC bool                     // When true, enables DIFC enforcement and requires sys___init call before tool access. Default is false for standard MCP client compatibility.
	Gateway    *GatewayConfig           // Gateway configuration (port, API key, etc.)
}

// GatewayConfig represents gateway-level configuration
type GatewayConfig struct {
	Port           int
	APIKey         string
	Domain         string
	StartupTimeout int // Seconds
	ToolTimeout    int // Seconds
}

// ServerConfig represents a single MCP server configuration
type ServerConfig struct {
	Command          string            `toml:"command"`
	Args             []string          `toml:"args"`
	Env              map[string]string `toml:"env"`
	WorkingDirectory string            `toml:"working_directory"`
}

// StdinConfig represents JSON configuration from stdin
type StdinConfig struct {
	MCPServers map[string]*StdinServerConfig `json:"mcpServers"`
	Gateway    *StdinGatewayConfig           `json:"gateway,omitempty"`
}

// StdinServerConfig represents a single server from stdin JSON
type StdinServerConfig struct {
	Type           string            `json:"type"` // "stdio" | "http" ("local" supported for backward compatibility)
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Container      string            `json:"container,omitempty"`
	Entrypoint     string            `json:"entrypoint,omitempty"`
	EntrypointArgs []string          `json:"entrypointArgs,omitempty"`
	Mounts         []string          `json:"mounts,omitempty"`
	URL            string            `json:"url,omitempty"` // For HTTP-based MCP servers
}

// StdinGatewayConfig represents gateway configuration from stdin JSON
type StdinGatewayConfig struct {
	Port           *int   `json:"port,omitempty"`
	APIKey         string `json:"apiKey,omitempty"`
	Domain         string `json:"domain,omitempty"`
	StartupTimeout *int   `json:"startupTimeout,omitempty"` // Seconds to wait for backend startup
	ToolTimeout    *int   `json:"toolTimeout,omitempty"`    // Seconds to wait for tool execution
}

// LoadFromFile loads configuration from a TOML file
func LoadFromFile(path string) (*Config, error) {
	logConfig.Printf("Loading configuration from file: path=%s", path)
	logger.LogInfo("startup", "Reading TOML configuration file: %s", path)
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		logger.LogError("startup", "Failed to decode TOML configuration: %v", err)
		return nil, fmt.Errorf("failed to decode TOML: %w", err)
	}
	logConfig.Printf("Successfully loaded %d servers from TOML file", len(cfg.Servers))
	logger.LogInfo("startup", "TOML configuration parsed successfully: %d servers found", len(cfg.Servers))
	return &cfg, nil
}

// LoadFromStdin loads configuration from stdin JSON
func LoadFromStdin() (*Config, error) {
	logConfig.Print("Loading configuration from stdin JSON")
	logger.LogInfo("startup", "Reading JSON configuration from stdin")
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logger.LogError("startup", "Failed to read from stdin: %v", err)
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	logConfig.Printf("Read %d bytes from stdin", len(data))
	logger.LogDebug("startup", "Read %d bytes from stdin", len(data))

	// Pre-process: normalize "local" type to "stdio" for backward compatibility
	// This must happen before schema validation since schema only accepts "stdio" or "http"
	data, err = normalizeLocalType(data)
	if err != nil {
		logger.LogError("startup", "Failed to normalize configuration: %v", err)
		return nil, fmt.Errorf("failed to normalize configuration: %w", err)
	}

	// Validate against JSON schema first (fail-fast, spec-compliant)
	logConfig.Print("Validating JSON schema")
	logger.LogDebug("startup", "Validating configuration against JSON schema")
	if err := validateJSONSchema(data); err != nil {
		logger.LogError("startup", "JSON schema validation failed: %v", err)
		return nil, err
	}
	logger.LogDebug("startup", "JSON schema validation passed")

	var stdinCfg StdinConfig
	if err := json.Unmarshal(data, &stdinCfg); err != nil {
		logger.LogError("startup", "Failed to parse JSON configuration: %v", err)
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	logConfig.Printf("Parsed stdin config with %d servers", len(stdinCfg.MCPServers))
	logger.LogInfo("startup", "JSON configuration parsed: %d servers found", len(stdinCfg.MCPServers))

	// Validate string patterns from schema (regex constraints)
	logConfig.Print("Validating string patterns")
	logger.LogDebug("startup", "Validating string patterns and constraints")
	if err := validateStringPatterns(&stdinCfg); err != nil {
		logger.LogError("startup", "String pattern validation failed: %v", err)
		return nil, err
	}

	// Validate gateway configuration (additional checks)
	if stdinCfg.Gateway != nil {
		logConfig.Print("Validating gateway configuration")
		logger.LogDebug("startup", "Validating gateway configuration")
	}
	if err := validateGatewayConfig(stdinCfg.Gateway); err != nil {
		logger.LogError("startup", "Gateway configuration validation failed: %v", err)
		return nil, err
	}

	// Convert stdin config to internal format
	cfg := &Config{
		Servers: make(map[string]*ServerConfig),
	}

	// Store gateway config with defaults
	if stdinCfg.Gateway != nil {
		cfg.Gateway = &GatewayConfig{
			Port:           3000,
			APIKey:         stdinCfg.Gateway.APIKey,
			Domain:         stdinCfg.Gateway.Domain,
			StartupTimeout: 60,
			ToolTimeout:    120,
		}
		if stdinCfg.Gateway.Port != nil {
			cfg.Gateway.Port = *stdinCfg.Gateway.Port
		}
		if stdinCfg.Gateway.StartupTimeout != nil {
			cfg.Gateway.StartupTimeout = *stdinCfg.Gateway.StartupTimeout
		}
		if stdinCfg.Gateway.ToolTimeout != nil {
			cfg.Gateway.ToolTimeout = *stdinCfg.Gateway.ToolTimeout
		}
	}

	for name, server := range stdinCfg.MCPServers {
		logConfig.Printf("Processing server config: %s", name)
		logger.LogDebug("startup", "Processing server configuration: %s, type: %s", name, server.Type)
		
		// Validate server configuration (fail-fast)
		if err := validateStdioServer(name, server); err != nil {
			logger.LogError("startup", "Server validation failed for %s: %v", name, err)
			return nil, err
		}

		// Expand variable expressions in env vars (fail-fast on undefined vars)
		if len(server.Env) > 0 {
			logConfig.Printf("Expanding %d environment variables for server: %s", len(server.Env), name)
			logger.LogDebug("startup", "Expanding environment variables for server: %s", name)
			expandedEnv, err := expandEnvVariables(server.Env, name)
			if err != nil {
				logger.LogError("startup", "Environment variable expansion failed for %s: %v", name, err)
				return nil, err
			}
			server.Env = expandedEnv
		}
		// Normalize type: "local" is an alias for "stdio" (backward compatibility)
		serverType := server.Type
		if serverType == "" {
			serverType = "stdio"
		}
		if serverType == "local" {
			serverType = "stdio"
		}

		// HTTP servers are not yet implemented
		if serverType == "http" {
			log.Printf("Warning: skipping server '%s' with type 'http' (HTTP transport not yet implemented)", name)
			logger.LogWarn("startup", "Skipping server '%s': HTTP transport not yet implemented", name)
			continue
		}

		// stdio/local servers only from this point
		// All stdio servers use Docker containers
		logConfig.Printf("Building Docker command for server: %s, container: %s", name, server.Container)
		logger.LogDebug("startup", "Building Docker command for server: %s, container: %s", name, server.Container)

		args := []string{
			"run",
			"--rm",
			"-i",
			// Standard environment variables for better Docker compatibility
			"-e", "NO_COLOR=1",
			"-e", "TERM=dumb",
			"-e", "PYTHONUNBUFFERED=1",
		}

		// Add entrypoint override if specified
		if server.Entrypoint != "" {
			args = append(args, "--entrypoint", server.Entrypoint)
		}

		// Add volume mounts if specified
		for _, mount := range server.Mounts {
			args = append(args, "-v", mount)
		}

		// Add user-specified environment variables
		// Empty string "" means passthrough from host (just -e KEY)
		// Non-empty string means explicit value (-e KEY=value)
		for k, v := range server.Env {
			args = append(args, "-e")
			if v == "" {
				// Passthrough from host environment
				args = append(args, k)
			} else {
				// Explicit value
				args = append(args, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Add container name
		args = append(args, server.Container)

		// Add entrypoint args
		args = append(args, server.EntrypointArgs...)

		cfg.Servers[name] = &ServerConfig{
			Command: "docker",
			Args:    args,
			Env:     make(map[string]string),
		}
		logger.LogInfo("startup", "Configured server: %s (container: %s)", name, server.Container)
	}

	logConfig.Printf("Converted stdin config to internal format with %d servers", len(cfg.Servers))
	logger.LogInfo("startup", "Configuration conversion complete: %d servers ready for initialization", len(cfg.Servers))
	return cfg, nil
}

// normalizeLocalType normalizes "local" type to "stdio" for backward compatibility
// This allows the configuration to pass schema validation which only accepts "stdio" or "http"
func normalizeLocalType(data []byte) ([]byte, error) {
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, err
	}

	// Check if mcpServers exists
	mcpServers, ok := rawConfig["mcpServers"]
	if !ok {
		return data, nil // No mcpServers, return as is
	}

	servers, ok := mcpServers.(map[string]interface{})
	if !ok {
		return data, nil // mcpServers is not a map, return as is
	}

	// Iterate through servers and normalize "local" to "stdio"
	modified := false
	for _, serverConfig := range servers {
		server, ok := serverConfig.(map[string]interface{})
		if !ok {
			continue
		}

		if typeVal, exists := server["type"]; exists {
			if typeStr, ok := typeVal.(string); ok && typeStr == "local" {
				server["type"] = "stdio"
				modified = true
			}
		}
	}

	// If we modified anything, re-marshal the data
	if modified {
		return json.Marshal(rawConfig)
	}

	return data, nil
}
