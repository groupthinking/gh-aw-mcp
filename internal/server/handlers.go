package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

// handleOAuthDiscovery returns a handler for OAuth discovery endpoint
// Returns 404 since the gateway doesn't use OAuth
func handleOAuthDiscovery() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s - OAuth discovery (not supported)", r.RemoteAddr, r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
}

// handleClose returns a handler for graceful shutdown endpoint (spec 5.1.3)
func handleClose(unifiedServer *UnifiedServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
}
