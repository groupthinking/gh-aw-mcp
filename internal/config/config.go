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
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode TOML: %w", err)
	}
	logConfig.Printf("Successfully loaded %d servers from TOML file", len(cfg.Servers))
	return &cfg, nil
}

// LoadFromStdin loads configuration from stdin JSON
func LoadFromStdin() (*Config, error) {
	logConfig.Print("Loading configuration from stdin JSON")
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	logConfig.Printf("Read %d bytes from stdin", len(data))

	// First unmarshal into a generic map to detect unknown fields (spec 4.3.1)
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check for unknown top-level fields
	knownFields := map[string]bool{
		"mcpServers": true,
		"gateway":    true,
	}
	for field := range rawConfig {
		if !knownFields[field] {
			return nil, fmt.Errorf("configuration error: unrecognized field '%s' at top level. Please check the specification version. Known fields are: mcpServers, gateway", field)
		}
	}

	var stdinCfg StdinConfig
	if err := json.Unmarshal(data, &stdinCfg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	logConfig.Printf("Parsed stdin config with %d servers", len(stdinCfg.MCPServers))

	// Validate gateway configuration first (fail-fast)
	if err := validateGatewayConfig(stdinCfg.Gateway); err != nil {
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
		// Validate server configuration (fail-fast)
		if err := validateStdioServer(name, server); err != nil {
			return nil, err
		}

		// Expand variable expressions in env vars (fail-fast on undefined vars)
		if len(server.Env) > 0 {
			expandedEnv, err := expandEnvVariables(server.Env, name)
			if err != nil {
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
			continue
		}

		// stdio/local servers only from this point
		// All stdio servers use Docker containers

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
	}

	logConfig.Printf("Converted stdin config to internal format with %d servers", len(cfg.Servers))
	return cfg, nil
}
