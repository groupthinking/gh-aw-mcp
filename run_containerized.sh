#!/bin/bash
# run_containerized.sh - Startup script for containerized MCP Gateway
# This script should be used when running the gateway inside a Docker container.
# It performs comprehensive validation of the container environment before starting.

set -e

# Color output for better visibility
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect container ID from /proc/self/cgroup
get_container_id() {
    if [ -f /proc/self/cgroup ]; then
        # Try to extract container ID from cgroup v1 or v2 paths
        local cid=$(cat /proc/self/cgroup | grep -oE '[0-9a-f]{12,64}' | head -1)
        if [ -n "$cid" ]; then
            echo "$cid"
            return 0
        fi
    fi
    
    # Fallback: check hostname (often set to container ID)
    if [ -f /.dockerenv ]; then
        hostname
        return 0
    fi
    
    return 1
}

# Verify we're running in a container
verify_containerized() {
    if [ ! -f /.dockerenv ] && ! grep -q 'docker\|containerd' /proc/self/cgroup 2>/dev/null; then
        log_error "This script should only be run inside a Docker container."
        log_error "For non-containerized deployments, use run.sh instead."
        exit 1
    fi
    log_info "Running in containerized environment"
}

# Check Docker daemon accessibility
check_docker_socket() {
    local socket_path="${DOCKER_HOST:-/var/run/docker.sock}"
    socket_path="${socket_path#unix://}"
    
    if [ ! -S "$socket_path" ]; then
        log_error "Docker socket not found at $socket_path"
        log_error "Mount the Docker socket: -v /var/run/docker.sock:/var/run/docker.sock"
        exit 1
    fi
    
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker daemon is not accessible"
        log_error "Ensure the Docker socket is properly mounted and accessible"
        exit 1
    fi
    
    log_info "Docker daemon is accessible"
}

# Validate required environment variables
check_required_env_vars() {
    local missing_vars=()
    
    if [ -z "$MCP_GATEWAY_PORT" ]; then
        missing_vars+=("MCP_GATEWAY_PORT")
    fi
    
    if [ -z "$MCP_GATEWAY_DOMAIN" ]; then
        missing_vars+=("MCP_GATEWAY_DOMAIN")
    fi
    
    if [ -z "$MCP_GATEWAY_API_KEY" ]; then
        missing_vars+=("MCP_GATEWAY_API_KEY")
    fi
    
    if [ ${#missing_vars[@]} -ne 0 ]; then
        log_error "Required environment variables not set:"
        for var in "${missing_vars[@]}"; do
            log_error "  - $var"
        done
        log_error ""
        log_error "Set these when running the container:"
        log_error "  docker run -e MCP_GATEWAY_PORT=8080 -e MCP_GATEWAY_DOMAIN=localhost -e MCP_GATEWAY_API_KEY=your-key ..."
        exit 1
    fi
    
    log_info "Required environment variables are set"
}

# Validate port mapping using docker inspect
validate_port_mapping() {
    local container_id="$1"
    local port="$MCP_GATEWAY_PORT"
    
    if [ -z "$container_id" ]; then
        log_warn "Cannot validate port mapping: container ID unknown"
        return 0
    fi
    
    local port_mapping=$(docker inspect --format '{{json .NetworkSettings.Ports}}' "$container_id" 2>/dev/null || echo "{}")
    
    if ! echo "$port_mapping" | grep -q "\"${port}/tcp\""; then
        log_error "Port $port is not exposed from the container"
        log_error "Add port mapping: -p <host_port>:$port"
        exit 1
    fi
    
    if ! echo "$port_mapping" | grep -q '"HostPort"'; then
        log_error "Port $port is exposed but not mapped to a host port"
        log_error "Add port mapping: -p <host_port>:$port"
        exit 1
    fi
    
    log_info "Port $port is properly mapped"
}

# Validate stdin is interactive (requires -i flag)
validate_stdin_interactive() {
    local container_id="$1"
    
    if [ -z "$container_id" ]; then
        log_warn "Cannot validate stdin: container ID unknown"
        return 0
    fi
    
    local stdin_open=$(docker inspect --format '{{.Config.OpenStdin}}' "$container_id" 2>/dev/null || echo "unknown")
    
    if [ "$stdin_open" != "true" ]; then
        log_error "Container was not started with -i flag"
        log_error "Stdin is required for passing JSON configuration"
        log_error "Start container with: docker run -i ..."
        exit 1
    fi
    
    log_info "Stdin is interactive"
}

# Validate container mounts and environment
validate_container_config() {
    local container_id="$1"
    
    if [ -z "$container_id" ]; then
        log_warn "Cannot validate container config: container ID unknown"
        return 0
    fi
    
    # Check for Docker socket mount
    local mounts=$(docker inspect --format '{{json .Mounts}}' "$container_id" 2>/dev/null || echo "[]")
    
    if ! echo "$mounts" | grep -q 'docker.sock'; then
        log_warn "Docker socket mount not detected in container mounts"
        log_warn "The gateway needs Docker access to spawn backend MCP servers"
    else
        log_info "Docker socket is mounted"
    fi
}

# Set DOCKER_API_VERSION based on architecture
set_docker_api_version() {
    local arch=$(uname -m)
    if [ "$arch" = "arm64" ] || [ "$arch" = "aarch64" ]; then
        export DOCKER_API_VERSION=1.43
    else
        export DOCKER_API_VERSION=1.44
    fi
    log_info "Set DOCKER_API_VERSION=$DOCKER_API_VERSION for $arch"
}

# Build command line arguments
build_command_args() {
    local host="${MCP_GATEWAY_HOST:-0.0.0.0}"
    local port="$MCP_GATEWAY_PORT"
    local mode="${MCP_GATEWAY_MODE:---routed}"
    
    local flags="$mode --listen ${host}:${port} --config-stdin"
    
    # Add env file if specified and exists
    if [ -n "$ENV_FILE" ] && [ -f "$ENV_FILE" ]; then
        flags="$flags --env $ENV_FILE"
        log_info "Using environment file: $ENV_FILE"
    fi
    
    echo "$flags"
}

# Main execution
main() {
    log_info "Starting MCP Gateway in containerized mode..."
    
    # Verify we're in a container
    verify_containerized
    
    # Get container ID
    CONTAINER_ID=$(get_container_id) || true
    if [ -n "$CONTAINER_ID" ]; then
        log_info "Container ID: ${CONTAINER_ID:0:12}..."
    else
        log_warn "Could not determine container ID"
    fi
    
    # Perform environment validation
    check_docker_socket
    check_required_env_vars
    set_docker_api_version
    
    # Perform container-specific validation
    if [ -n "$CONTAINER_ID" ]; then
        validate_port_mapping "$CONTAINER_ID"
        validate_stdin_interactive "$CONTAINER_ID"
        validate_container_config "$CONTAINER_ID"
    fi
    
    # Build command
    FLAGS=$(build_command_args)
    CMD="./awmg"
    
    log_info "Command: $CMD $FLAGS"
    log_info "Waiting for JSON configuration on stdin..."
    log_info ""
    log_info "IMPORTANT: Configuration must be provided via stdin."
    log_info "No default configuration is used in containerized mode."
    log_info ""
    
    # Execute - stdin will be passed through
    exec $CMD $FLAGS
}

main "$@"
