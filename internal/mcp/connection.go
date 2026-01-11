package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logConn = logger.New("mcp:connection")

// Connection represents a connection to an MCP server using the official SDK
type Connection struct {
	client  *sdk.Client
	session *sdk.ClientSession
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewConnection creates a new MCP connection using the official SDK
func NewConnection(ctx context.Context, command string, args []string, env map[string]string) (*Connection, error) {
	logger.LogInfo("backend", "Creating new MCP backend connection, command=%s, args=%v", command, args)
	logConn.Printf("Creating new MCP connection: command=%s, args=%v", command, args)
	ctx, cancel := context.WithCancel(ctx)

	// Create MCP client
	client := sdk.NewClient(&sdk.Implementation{
		Name:    "awmg",
		Version: "1.0.0",
	}, nil)

	// Expand Docker -e flags that reference environment variables
	// Docker's `-e VAR_NAME` expects VAR_NAME to be in the environment
	expandedArgs := expandDockerEnvArgs(args)
	logConn.Printf("Expanded args for Docker env: %v", expandedArgs)

	// Create command transport
	cmd := exec.CommandContext(ctx, command, expandedArgs...)

	// Start with parent's environment to inherit shell variables
	cmd.Env = append([]string{}, cmd.Environ()...)

	// Add/override with config-specified environment variables
	if len(env) > 0 {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	logger.LogInfo("backend", "Starting MCP backend server, command=%s, args=%v", command, expandedArgs)
	log.Printf("Starting MCP server command: %s %v", command, expandedArgs)
	transport := &sdk.CommandTransport{Command: cmd}

	// Connect to the server (this handles the initialization handshake automatically)
	log.Printf("Connecting to MCP server...")
	logConn.Print("Initiating MCP server connection and handshake")
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		cancel()

		// Enhanced error context for debugging
		logger.LogErrorMd("backend", "MCP backend connection failed, command=%s, args=%v, error=%v", command, expandedArgs, err)
		log.Printf("❌ MCP Connection Failed:")
		log.Printf("   Command: %s", command)
		log.Printf("   Args: %v", expandedArgs)
		log.Printf("   Error: %v", err)

		// Check if it's a command not found error
		if strings.Contains(err.Error(), "executable file not found") ||
			strings.Contains(err.Error(), "no such file or directory") {
			logger.LogErrorMd("backend", "MCP backend command not found, command=%s", command)
			log.Printf("   ⚠️  Command '%s' not found in PATH", command)
			log.Printf("   ⚠️  Verify the command is installed and executable")
		}

		// Check if it's a connection/protocol error
		if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "broken pipe") {
			logger.LogErrorMd("backend", "MCP backend connection/protocol error, command=%s", command)
			log.Printf("   ⚠️  Process started but terminated unexpectedly")
			log.Printf("   ⚠️  Check if the command supports MCP protocol over stdio")
		}

		logConn.Printf("Connection failed: command=%s, error=%v", command, err)
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	logger.LogInfoMd("backend", "Successfully connected to MCP backend server, command=%s", command)
	logConn.Printf("Successfully connected to MCP server: command=%s", command)

	conn := &Connection{
		client:  client,
		session: session,
		ctx:     ctx,
		cancel:  cancel,
	}

	log.Printf("Started MCP server: %s %v", command, args)
	return conn, nil
}

// SendRequest sends a JSON-RPC request and waits for the response
// The serverID parameter is used for logging to associate the request with a backend server
func (c *Connection) SendRequest(method string, params interface{}) (*Response, error) {
	return c.SendRequestWithServerID(method, params, "unknown")
}

// SendRequestWithServerID sends a JSON-RPC request with server ID for logging
func (c *Connection) SendRequestWithServerID(method string, params interface{}, serverID string) (*Response, error) {
	// Log the outbound request to backend server
	requestPayload, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	logger.LogRPCRequest(logger.RPCDirectionOutbound, serverID, method, requestPayload)
	
	var result *Response
	var err error
	
	switch method {
	case "tools/list":
		result, err = c.listTools()
	case "tools/call":
		result, err = c.callTool(params)
	case "resources/list":
		result, err = c.listResources()
	case "resources/read":
		result, err = c.readResource(params)
	case "prompts/list":
		result, err = c.listPrompts()
	case "prompts/get":
		result, err = c.getPrompt(params)
	default:
		err = fmt.Errorf("unsupported method: %s", method)
	}
	
	// Log the response from backend server
	var responsePayload []byte
	if result != nil {
		responsePayload, _ = json.Marshal(result)
	}
	logger.LogRPCResponse(logger.RPCDirectionInbound, serverID, responsePayload, err)
	
	return result, err
}

func (c *Connection) listTools() (*Response, error) {
	result, err := c.session.ListTools(c.ctx, &sdk.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1, // Placeholder ID
		Result:  resultJSON,
	}, nil
}

func (c *Connection) callTool(params interface{}) (*Response, error) {
	var callParams CallToolParams
	paramsJSON, _ := json.Marshal(params)
	if err := json.Unmarshal(paramsJSON, &callParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	result, err := c.session.CallTool(c.ctx, &sdk.CallToolParams{
		Name:      callParams.Name,
		Arguments: callParams.Arguments,
	})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultJSON,
	}, nil
}

func (c *Connection) listResources() (*Response, error) {
	result, err := c.session.ListResources(c.ctx, &sdk.ListResourcesParams{})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultJSON,
	}, nil
}

func (c *Connection) readResource(params interface{}) (*Response, error) {
	var readParams struct {
		URI string `json:"uri"`
	}
	paramsJSON, _ := json.Marshal(params)
	if err := json.Unmarshal(paramsJSON, &readParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	result, err := c.session.ReadResource(c.ctx, &sdk.ReadResourceParams{
		URI: readParams.URI,
	})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultJSON,
	}, nil
}

func (c *Connection) listPrompts() (*Response, error) {
	result, err := c.session.ListPrompts(c.ctx, &sdk.ListPromptsParams{})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultJSON,
	}, nil
}

func (c *Connection) getPrompt(params interface{}) (*Response, error) {
	var getParams struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	paramsJSON, _ := json.Marshal(params)
	if err := json.Unmarshal(paramsJSON, &getParams); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	result, err := c.session.GetPrompt(c.ctx, &sdk.GetPromptParams{
		Name:      getParams.Name,
		Arguments: getParams.Arguments,
	})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  resultJSON,
	}, nil
}

// expandDockerEnvArgs expands Docker -e flags that reference environment variables
// Converts "-e VAR_NAME" to "-e VAR_NAME=value" by reading from the process environment
func expandDockerEnvArgs(args []string) []string {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check if this is a -e flag
		if arg == "-e" && i+1 < len(args) {
			nextArg := args[i+1]
			// If next arg doesn't contain '=', it's a variable reference
			if len(nextArg) > 0 && !containsEqual(nextArg) {
				// Look up the variable in the environment
				if value, exists := os.LookupEnv(nextArg); exists {
					result = append(result, "-e")
					result = append(result, fmt.Sprintf("%s=%s", nextArg, value))
					i++ // Skip the next arg since we processed it
					continue
				}
			}
		}
		result = append(result, arg)
	}
	return result
}

func containsEqual(s string) bool {
	for _, c := range s {
		if c == '=' {
			return true
		}
	}
	return false
}

// Close closes the connection
func (c *Connection) Close() error {
	c.cancel()
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
