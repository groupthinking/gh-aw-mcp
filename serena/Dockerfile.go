# Serena MCP Server - Go Language Support
FROM golang:1.23-alpine AS base

# Install necessary tools
RUN apk add --no-cache git python3 py3-pip

# Install Serena from GitHub
RUN pip3 install --break-system-packages git+https://github.com/oraios/serena

# Install gopls (Go language server)
RUN go install golang.org/x/tools/gopls@latest

# Set up working directory
WORKDIR /workspace

# Default environment variables
ENV SERENA_CONTEXT=codex
ENV SERENA_PROJECT=/workspace

# Entrypoint runs the Serena MCP server with Go support
ENTRYPOINT ["serena", "start-mcp-server", "--context", "${SERENA_CONTEXT}", "--project", "${SERENA_PROJECT}"]
