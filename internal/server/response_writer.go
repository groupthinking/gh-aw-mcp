package server

import (
	"bytes"
	"net/http"
)

// responseWriter wraps http.ResponseWriter to capture response body and status code
// This unified implementation replaces loggingResponseWriter and sdkLoggingResponseWriter
type responseWriter struct {
	http.ResponseWriter
	body       bytes.Buffer
	statusCode int
}

// newResponseWriter creates a new responseWriter with default status code
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Body returns the captured response body as bytes
func (w *responseWriter) Body() []byte {
	return w.body.Bytes()
}

// StatusCode returns the captured HTTP status code
func (w *responseWriter) StatusCode() int {
	return w.statusCode
}
