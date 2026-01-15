package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logTransport = logger.New("server:transport")

// HTTPTransport wraps the SDK's HTTP transport
type HTTPTransport struct {
	Addr string
}

// Start implements sdk.Transport interface
func (t *HTTPTransport) Start(ctx context.Context) error {
	logTransport.Printf("Starting HTTP transport: addr=%s", t.Addr)
	// The SDK will handle the actual HTTP server setup
	// We just need to provide the address
	log.Printf("HTTP transport ready on %s", t.Addr)
	return nil
}

// Send implements sdk.Transport interface
func (t *HTTPTransport) Send(msg interface{}) error {
	// Messages are sent via HTTP responses, handled by SDK
	return nil
}

// Recv implements sdk.Transport interface
func (t *HTTPTransport) Recv() (interface{}, error) {
	// Messages are received via HTTP requests, handled by SDK
	return nil, nil
}

// Close implements sdk.Transport interface
func (t *HTTPTransport) Close() error {
	return nil
}

// loggingResponseWriter wraps http.ResponseWriter to capture response body
type loggingResponseWriter struct {
	http.ResponseWriter
	body       []byte
	statusCode int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// withResponseLogging wraps an http.Handler to log response bodies
func withResponseLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lw := &loggingResponseWriter{ResponseWriter: w, body: []byte{}, statusCode: http.StatusOK}
		handler.ServeHTTP(lw, r)
		if len(lw.body) > 0 {
			log.Printf("[%s] %s %s - Status: %d, Response: %s", r.RemoteAddr, r.Method, r.URL.Path, lw.statusCode, string(lw.body))
		}
	})
}

// CreateHTTPServerForMCP creates an HTTP server that handles MCP over streamable HTTP transport
// If apiKey is provided, all requests except /health require authentication (spec 7.1)
func CreateHTTPServerForMCP(addr string, unifiedServer *UnifiedServer, apiKey string) *http.Server {
	logTransport.Printf("Creating HTTP server for MCP: addr=%s, auth_enabled=%v", addr, apiKey != "")
	mux := http.NewServeMux()

	// OAuth discovery endpoint - return 404 since we don't use OAuth
	oauthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s - OAuth discovery (not supported)", r.RemoteAddr, r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	mux.Handle("/mcp/.well-known/oauth-authorization-server", withResponseLogging(oauthHandler))

	logTransport.Print("Registering streamable HTTP handler for MCP protocol")
	// Create StreamableHTTP handler for MCP protocol (supports POST requests)
	// This is what Codex uses with transport = "streamablehttp"
	streamableHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		// With streamable HTTP, this callback fires for each new session establishment
		// Subsequent JSON-RPC messages in the same session are handled by the SDK
		// We use the Authorization header value as the session ID
		// This groups all requests from the same agent (same auth value) into one session

		// Extract session ID from Authorization header
		// Per spec 7.1: When API key is configured, Authorization contains plain API key
		// When API key is not configured, supports Bearer token for backward compatibility
		authHeader := r.Header.Get("Authorization")
		var sessionID string

		if strings.HasPrefix(authHeader, "Bearer ") {
			// Bearer token format (for backward compatibility when no API key)
			sessionID = strings.TrimPrefix(authHeader, "Bearer ")
			sessionID = strings.TrimSpace(sessionID)
		} else if authHeader != "" {
			// Plain format (per spec 7.1 - API key is session ID)
			sessionID = authHeader
		}

		// Reject requests without Authorization header
		if sessionID == "" {
			logTransport.Printf("Rejecting connection: no Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			logger.LogErrorMd("client", "MCP connection rejected: no Authorization header, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
			log.Printf("[%s] %s %s - REJECTED: No Authorization header", r.RemoteAddr, r.Method, r.URL.Path)
			// Return nil to reject the connection
			// The SDK will handle sending an appropriate error response
			return nil
		}

		logger.LogInfo("client", "MCP connection established, remote=%s, method=%s, path=%s, session=%s", r.RemoteAddr, r.Method, r.URL.Path, sessionID)
		log.Printf("=== NEW STREAMABLE HTTP CONNECTION ===")
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		log.Printf("Authorization (Session ID): %s", sessionID)

		log.Printf("DEBUG: About to check request body, Method=%s, Body!=nil: %v", r.Method, r.Body != nil)

		// Log request body for debugging (typically the 'initialize' request)
		if r.Method == "POST" && r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				logger.LogDebug("client", "MCP initialize request body, session=%s, body=%s", sessionID, string(bodyBytes))
				log.Printf("Request body: %s", string(bodyBytes))
				// Restore body
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Store session ID in request context
		// This context will be passed to all tool handlers for this connection
		ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
		*r = *r.WithContext(ctx)
		log.Printf("✓ Injected session ID into context")
		log.Printf("==========================\n")

		return unifiedServer.server
	}, &sdk.StreamableHTTPOptions{
		Stateless: false, // Support stateful sessions
	})

	// Apply auth middleware if API key is configured (spec 7.1)
	var finalHandler http.Handler = streamableHandler
	if apiKey != "" {
		finalHandler = authMiddleware(apiKey, streamableHandler.ServeHTTP)
	}

	// Mount handler at /mcp endpoint (logging is done in the callback above)
	mux.Handle("/mcp/", finalHandler)
	mux.Handle("/mcp", finalHandler)

	// Health check (spec 8.1.1)
	healthHandler := HandleHealth(unifiedServer)
	mux.Handle("/health", withResponseLogging(healthHandler))

	// Close endpoint for graceful shutdown (spec 5.1.3)
	closeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logTransport.Printf("Close endpoint called: method=%s, remote=%s", r.Method, r.RemoteAddr)
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		logger.LogInfo("shutdown", "Close endpoint called, remote=%s", r.RemoteAddr)

		// Only accept POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if already closed (idempotency - spec 5.1.3)
		if unifiedServer.IsShutdown() {
			logger.LogWarn("shutdown", "Close endpoint called but gateway already closed, remote=%s", r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGone) // 410 Gone
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Gateway has already been closed",
			})
			return
		}

		// Initiate shutdown and get server count
		serversTerminated := unifiedServer.InitiateShutdown()
		logTransport.Printf("Shutdown initiated: servers_terminated=%d", serversTerminated)

		// Return success response (spec 5.1.3)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"status":            "closed",
			"message":           "Gateway shutdown initiated",
			"serversTerminated": serversTerminated,
		}
		json.NewEncoder(w).Encode(response)

		logger.LogInfo("shutdown", "Close endpoint response sent, servers_terminated=%d", serversTerminated)
		log.Printf("Gateway shutdown initiated. Terminated %d server(s)", serversTerminated)

		// Exit the process after a brief delay to ensure response is sent
		// Skip exit in test mode
		if unifiedServer.ShouldExit() {
			go func() {
				time.Sleep(100 * time.Millisecond)
				logger.LogInfo("shutdown", "Gateway process exiting with status 0")
				os.Exit(0)
			}()
		}
	})

	// Apply auth middleware if API key is configured (spec 7.1)
	var finalCloseHandler http.Handler = closeHandler
	if apiKey != "" {
		finalCloseHandler = authMiddleware(apiKey, closeHandler.ServeHTTP)
	}
	mux.Handle("/close", withResponseLogging(finalCloseHandler))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
