# Serena MCP Server Container Image
# Provides AI-powered code intelligence with support for Python, Java, JavaScript/TypeScript, and Go

FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies required for language servers and build tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Build essentials
    build-essential \
    git \
    curl \
    wget \
    # Java Runtime Environment (for Java language support)
    default-jdk \
    # Node.js (for JavaScript/TypeScript language support)
    nodejs \
    npm \
    # Go runtime (for Go language support)
    golang-go \
    # Additional utilities
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

# Set environment variables for language runtimes
ENV JAVA_HOME=/usr/lib/jvm/default-java
ENV PATH="${JAVA_HOME}/bin:${PATH}"
ENV GOPATH=/go
ENV PATH="${GOPATH}/bin:/usr/bin:${PATH}"

# Install Serena MCP server from GitHub using pip
# Serena is a Python package, so we can install it directly
RUN pip install --no-cache-dir git+https://github.com/oraios/serena.git || \
    pip install --no-cache-dir serena-agent

# Install common language servers that Serena may need
# These are installed globally to be available to Serena
RUN npm install -g \
    typescript \
    typescript-language-server \
    @vscode/java-language-server \
    && rm -rf /root/.npm

# Install Python language server (pyright is included in Serena dependencies)
# Additional Python tools
RUN pip install --no-cache-dir \
    python-lsp-server \
    pylsp-mypy

# Install gopls (Go language server)
RUN go install golang.org/x/tools/gopls@latest

# Create directories for Serena configuration and caching
RUN mkdir -p /workspace /tmp/serena-cache /root/.serena

# Set environment variables for Serena
ENV SERENA_WORKSPACE=/workspace
ENV SERENA_CACHE_DIR=/tmp/serena-cache

# Expose the workspace directory as a volume mount point
VOLUME ["/workspace"]

# Set default entrypoint to run Serena MCP server
# This will start the MCP server on stdio transport
ENTRYPOINT ["serena-mcp-server"]

# Default arguments (can be overridden)
CMD []
