package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logConn = logger.New("mcp:connection")

// ContextKey for session ID
type ContextKey string

// SessionIDContextKey is used to store MCP session ID in context
// This is the same key used in the server package to avoid circular dependencies
const SessionIDContextKey ContextKey = "awmg-session-id"

// requestIDCounter is used to generate unique request IDs for HTTP requests
var requestIDCounter uint64

// Connection represents a connection to an MCP server using the official SDK
type Connection struct {
	client  *sdk.Client
	session *sdk.ClientSession
	ctx     context.Context
	cancel  context.CancelFunc
	// HTTP-specific fields
	isHTTP        bool
	httpURL       string
	headers       map[string]string
	httpClient    *http.Client
	httpSessionID string // Session ID returned by the HTTP backend
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
		isHTTP:  false,
	}

	log.Printf("Started MCP server: %s %v", command, args)
	return conn, nil
}

// NewHTTPConnection creates a new HTTP-based MCP connection
// For HTTP servers that are already running, we connect and initialize a session
func NewHTTPConnection(ctx context.Context, url string, headers map[string]string) (*Connection, error) {
	logger.LogInfo("backend", "Creating HTTP MCP connection, url=%s", url)
	logConn.Printf("Creating HTTP MCP connection: url=%s", url)
	ctx, cancel := context.WithCancel(ctx)

	// Create an HTTP client with appropriate timeouts
	httpClient := &http.Client{
		Timeout: 120 * time.Second, // Overall request timeout
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	conn := &Connection{
		ctx:        ctx,
		cancel:     cancel,
		isHTTP:     true,
		httpURL:    url,
		headers:    headers,
		httpClient: httpClient,
	}

	// Send initialize request to establish a session with the HTTP backend
	// This is critical for backends that require session management
	logConn.Printf("Sending initialize request to HTTP backend: url=%s", url)
	sessionID, err := conn.initializeHTTPSession()
	if err != nil {
		cancel()
		logger.LogError("backend", "Failed to initialize HTTP session, url=%s, error=%v", url, err)
		return nil, fmt.Errorf("failed to initialize HTTP session: %w", err)
	}

	conn.httpSessionID = sessionID
	logger.LogInfo("backend", "Successfully created HTTP MCP connection with session, url=%s, session=%s", url, sessionID)
	logConn.Printf("HTTP connection created with session: url=%s, session=%s", url, sessionID)
	log.Printf("Configured HTTP MCP server with session: %s (session: %s)", url, sessionID)
	return conn, nil
}

// IsHTTP returns true if this is an HTTP connection
func (c *Connection) IsHTTP() bool {
	return c.isHTTP
}

// GetHTTPURL returns the HTTP URL for this connection
func (c *Connection) GetHTTPURL() string {
	return c.httpURL
}

// GetHTTPHeaders returns the HTTP headers for this connection
func (c *Connection) GetHTTPHeaders() map[string]string {
	return c.headers
}

// SendRequest sends a JSON-RPC request and waits for the response
// The serverID parameter is used for logging to associate the request with a backend server
func (c *Connection) SendRequest(method string, params interface{}) (*Response, error) {
	return c.SendRequestWithServerID(context.Background(), method, params, "unknown")
}

// SendRequestWithServerID sends a JSON-RPC request with server ID for logging
// The ctx parameter is used to extract session ID for HTTP MCP servers
func (c *Connection) SendRequestWithServerID(ctx context.Context, method string, params interface{}, serverID string) (*Response, error) {
	// Log the outbound request to backend server
	requestPayload, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	logger.LogRPCRequest(logger.RPCDirectionOutbound, serverID, method, requestPayload)

	var result *Response
	var err error

	// Handle HTTP connections by proxying the request
	if c.isHTTP {
		result, err = c.sendHTTPRequest(ctx, method, params)
		// Log the response from backend server
		var responsePayload []byte
		if result != nil {
			responsePayload, _ = json.Marshal(result)
		}
		logger.LogRPCResponse(logger.RPCDirectionInbound, serverID, responsePayload, err)
		return result, err
	}

	// Handle stdio connections using SDK client
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

// initializeHTTPSession sends an initialize request to the HTTP backend and captures the session ID
func (c *Connection) initializeHTTPSession() (string, error) {
	// Generate unique request ID
	requestID := atomic.AddUint64(&requestIDCounter, 1)

	// Create initialize request with MCP protocol parameters
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "awmg",
			"version": "1.0.0",
		},
	}

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "initialize",
		"params":  initParams,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	logConn.Printf("Sending initialize request: %s", string(requestBody))

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.httpURL, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Generate a temporary session ID for the initialize request
	// Some backends may require this header even during initialization
	tempSessionID := fmt.Sprintf("awmg-init-%d", requestID)
	httpReq.Header.Set("Mcp-Session-Id", tempSessionID)
	logConn.Printf("Sending initialize with temporary session ID: %s", tempSessionID)

	// Add configured headers (e.g., Authorization)
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}

	logConn.Printf("Sending initialize to %s", c.httpURL)

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send initialize request: %w", err)
	}
	defer httpResp.Body.Close()

	// Capture the Mcp-Session-Id from response headers
	sessionID := httpResp.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		logConn.Printf("Captured Mcp-Session-Id from response: %s", sessionID)
	} else {
		// If no session ID in response, use the temporary one
		// This handles backends that don't return a session ID
		sessionID = tempSessionID
		logConn.Printf("No Mcp-Session-Id in response, using temporary session ID: %s", sessionID)
	}

	// Read response body
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read initialize response: %w", err)
	}

	logConn.Printf("Initialize response: status=%d, body_len=%d, session=%s", httpResp.StatusCode, len(responseBody), sessionID)

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("initialize failed: status=%d, body=%s", httpResp.StatusCode, string(responseBody))
	}

	// Parse JSON-RPC response to check for errors
	var rpcResponse Response
	if err := json.Unmarshal(responseBody, &rpcResponse); err != nil {
		return "", fmt.Errorf("failed to parse initialize response: %w", err)
	}

	if rpcResponse.Error != nil {
		return "", fmt.Errorf("initialize error: code=%d, message=%s", rpcResponse.Error.Code, rpcResponse.Error.Message)
	}

	return sessionID, nil
}

// sendHTTPRequest sends a JSON-RPC request to an HTTP MCP server
// The ctx parameter is used to extract session ID for the Mcp-Session-Id header
func (c *Connection) sendHTTPRequest(ctx context.Context, method string, params interface{}) (*Response, error) {
	// Generate unique request ID using atomic counter
	requestID := atomic.AddUint64(&requestIDCounter, 1)

	// Create JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  method,
		"params":  params,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.httpURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Add Mcp-Session-Id header with priority:
	// 1) Context session ID (if explicitly provided for this request)
	// 2) Stored httpSessionID from initialization
	var sessionID string
	if ctxSessionID, ok := ctx.Value(SessionIDContextKey).(string); ok && ctxSessionID != "" {
		sessionID = ctxSessionID
		logConn.Printf("Using session ID from context: %s", sessionID)
	} else if c.httpSessionID != "" {
		sessionID = c.httpSessionID
		logConn.Printf("Using stored session ID from initialization: %s", sessionID)
	}

	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	} else {
		logConn.Printf("No session ID available (backend may not require session management)")
	}

	// Add configured headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}

	logConn.Printf("Sending HTTP request to %s: method=%s, id=%d", c.httpURL, method, requestID)

	// Send request using the reusable HTTP client
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response: %w", err)
	}

	logConn.Printf("Received HTTP response: status=%d, body_len=%d", httpResp.StatusCode, len(responseBody))

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: status=%d, body=%s", httpResp.StatusCode, string(responseBody))
	}

	// Parse JSON-RPC response
	var rpcResponse Response
	if err := json.Unmarshal(responseBody, &rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	return &rpcResponse, nil
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
