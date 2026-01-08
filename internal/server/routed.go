package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var logRouted = logger.New("server:routed")

// CreateHTTPServerForRoutedMode creates an HTTP server for routed mode
// In routed mode, each backend is accessible at /mcp/<server>
// Multiple routes from the same Bearer token share a session
// If apiKey is provided, all requests except /health require authentication (spec 7.1)
func CreateHTTPServerForRoutedMode(addr string, unifiedServer *UnifiedServer, apiKey string) *http.Server {
	logRouted.Printf("Creating HTTP server for routed mode: addr=%s", addr)
	mux := http.NewServeMux()

	// OAuth discovery endpoint - return 404 since we don't use OAuth
	oauthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s - OAuth discovery (not supported)", r.RemoteAddr, r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	mux.Handle("/mcp/.well-known/oauth-authorization-server", withResponseLogging(oauthHandler))

	// Create routes for all backends plus sys
	allBackends := append([]string{"sys"}, unifiedServer.GetServerIDs()...)
	logRouted.Printf("Registering routes for %d backends: %v", len(allBackends), allBackends)

	// Create a proxy for each backend server (including sys)
	for _, serverID := range allBackends {
		// Capture serverID for the closure
		backendID := serverID
		route := fmt.Sprintf("/mcp/%s", backendID)

		// Create StreamableHTTP handler for this route
		routeHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			var sessionID string

			if strings.HasPrefix(authHeader, "Bearer ") {
				sessionID = strings.TrimPrefix(authHeader, "Bearer ")
				sessionID = strings.TrimSpace(sessionID)
			}

			// Reject requests without valid Bearer token
			if sessionID == "" {
				logger.LogError("client", "Rejected MCP client connection: no Bearer token, remote=%s, path=%s", r.RemoteAddr, r.URL.Path)
				log.Printf("[%s] %s %s - REJECTED: No Bearer token", r.RemoteAddr, r.Method, r.URL.Path)
				return nil
			}

			logger.LogInfo("client", "New MCP client connection, remote=%s, method=%s, path=%s, backend=%s, session=%s",
				r.RemoteAddr, r.Method, r.URL.Path, backendID, sessionID)
			log.Printf("=== NEW SSE CONNECTION (ROUTED) ===")
			log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
			log.Printf("Backend: %s", backendID)
			log.Printf("Bearer Token (Session ID): %s", sessionID)

			// Log request body for debugging
			if r.Method == "POST" && r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil && len(bodyBytes) > 0 {
					logger.LogDebug("client", "MCP client request body, backend=%s, body=%s", backendID, string(bodyBytes))
					log.Printf("Request body: %s", string(bodyBytes))
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}

			// Store session ID and backend ID in request context
			ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
			ctx = context.WithValue(ctx, ContextKey("backend-id"), backendID)
			*r = *r.WithContext(ctx)
			log.Printf("✓ Injected session ID and backend ID into context")
			log.Printf("===================================\n")

			// Return a filtered proxy server that only exposes this backend's tools
			return createFilteredServer(unifiedServer, backendID)
		}, &sdk.StreamableHTTPOptions{
			Stateless: false,
		})

		// Apply auth middleware if API key is configured (spec 7.1)
		var finalHandler http.Handler = routeHandler
		if apiKey != "" {
			finalHandler = authMiddleware(apiKey, routeHandler.ServeHTTP)
		}

		// Mount the handler at both /mcp/<server> and /mcp/<server>/
		mux.Handle(route+"/", finalHandler)
		mux.Handle(route, finalHandler)
		log.Printf("Registered route: %s", route)
	}

	// Health check
	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})
	mux.Handle("/health", withResponseLogging(healthHandler))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

// createFilteredServer creates an MCP server that only exposes tools for a specific backend
// This reuses the unified server's tool handlers, ensuring all calls go through the same session
func createFilteredServer(unifiedServer *UnifiedServer, backendID string) *sdk.Server {
	logRouted.Printf("Creating filtered server: backend=%s", backendID)

	// Create a new SDK server for this route
	server := sdk.NewServer(&sdk.Implementation{
		Name:    fmt.Sprintf("awmg-%s", backendID),
		Version: "1.0.0",
	}, nil)

	// Get tools for this backend from the unified server
	tools := unifiedServer.GetToolsForBackend(backendID)

	log.Printf("Creating filtered server for %s with %d tools", backendID, len(tools))
	logRouted.Printf("Backend %s has %d tools available", backendID, len(tools))

	// Register each tool (without prefix) using the unified server's handlers
	for _, toolInfo := range tools {
		// Capture for closure
		toolNameCopy := toolInfo.Name

		// Get the unified server's handler for this tool
		handler := unifiedServer.GetToolHandler(backendID, toolInfo.Name)
		if handler == nil {
			log.Printf("WARNING: No handler found for %s___%s", backendID, toolInfo.Name)
			continue
		}

		sdk.AddTool(server, &sdk.Tool{
			Name:        toolInfo.Name, // Without prefix for the client
			Description: toolInfo.Description,
			InputSchema: toolInfo.InputSchema,
		}, func(ctx context.Context, req *sdk.CallToolRequest, args interface{}) (*sdk.CallToolResult, interface{}, error) {
			// Call the unified server's handler directly
			// This ensures we go through the same session and connection pool
			log.Printf("[ROUTED] Calling unified handler for: %s", toolNameCopy)
			return handler(ctx, req, args)
		})
	}

	return server
}
