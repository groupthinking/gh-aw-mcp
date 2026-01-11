package server

import (
	"encoding/json"
	"net/http"
)

// HealthResponse represents the JSON structure for the /health endpoint response
// as defined in MCP Gateway Specification section 8.1.1
type HealthResponse struct {
	Status         string                  `json:"status"`         // "healthy" or "unhealthy"
	SpecVersion    string                  `json:"specVersion"`    // MCP Gateway Specification version
	GatewayVersion string                  `json:"gatewayVersion"` // Gateway implementation version
	Servers        map[string]ServerStatus `json:"servers"`        // Map of server names to their health status
}

// BuildHealthResponse constructs a HealthResponse from the unified server's status
func BuildHealthResponse(unifiedServer *UnifiedServer) HealthResponse {
	// Get server status
	serverStatus := unifiedServer.GetServerStatus()

	// Determine overall health based on server status
	overallStatus := "healthy"
	for _, status := range serverStatus {
		if status.Status == "error" {
			overallStatus = "unhealthy"
			break
		}
	}

	return HealthResponse{
		Status:         overallStatus,
		SpecVersion:    MCPGatewaySpecVersion,
		GatewayVersion: gatewayVersion,
		Servers:        serverStatus,
	}
}

// HandleHealth returns an http.HandlerFunc that handles the /health endpoint
// This function is used by both routed and unified modes to ensure consistent behavior
func HandleHealth(unifiedServer *UnifiedServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := BuildHealthResponse(unifiedServer)
		json.NewEncoder(w).Encode(response)
	}
}
