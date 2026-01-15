# Serena MCP Server - Multi-Language Support (All Languages)
# This unified container includes support for Go, TypeScript, Python, Java, Rust, and C#
# 
# WARNING: This image is significantly larger (~2-4GB) than language-specific images
# Consider using language-specific images (Dockerfile-go, Dockerfile-typescript, etc.)
# if you only need support for one language.

FROM ubuntu:22.04 AS base

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install base system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release \
    software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# Install Python 3.12
RUN add-apt-repository ppa:deadsnakes/ppa -y && \
    apt-get update && \
    apt-get install -y \
    python3.12 \
    python3.12-dev \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Set Python 3.12 as default
RUN update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.12 1 && \
    update-alternatives --install /usr/bin/python python /usr/bin/python3.12 1

# Install Serena from GitHub
RUN pip3 install --break-system-packages git+https://github.com/oraios/serena

# Install Python language server
RUN pip3 install --break-system-packages python-lsp-server

# Install Node.js 22 (for TypeScript)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Install TypeScript and language server
RUN npm install -g typescript typescript-language-server

# Install Go 1.23
RUN wget -q https://go.dev/dl/go1.23.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz && \
    rm go1.23.0.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/root/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Install gopls (Go language server)
RUN go install golang.org/x/tools/gopls@latest

# Install Java 21 (Eclipse Temurin)
RUN wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \
    echo "deb https://packages.adoptium.net/artifactory/deb $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/adoptium.list && \
    apt-get update && \
    apt-get install -y temurin-21-jdk && \
    rm -rf /var/lib/apt/lists/*

# Install Eclipse JDT Language Server
RUN mkdir -p /opt/jdtls && \
    wget -q https://download.eclipse.org/jdtls/snapshots/jdt-language-server-latest.tar.gz && \
    tar -xzf jdt-language-server-latest.tar.gz -C /opt/jdtls && \
    rm jdt-language-server-latest.tar.gz

ENV PATH="/opt/jdtls/bin:${PATH}"

# Install Rust 1.75
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain 1.75.0 && \
    . "$HOME/.cargo/env"

ENV PATH="/root/.cargo/bin:${PATH}"

# Install rust-analyzer (Rust language server)
RUN rustup component add rust-analyzer

# Install .NET SDK 8.0 (for C#)
RUN wget -q https://packages.microsoft.com/config/ubuntu/22.04/packages-microsoft-prod.deb -O packages-microsoft-prod.deb && \
    dpkg -i packages-microsoft-prod.deb && \
    rm packages-microsoft-prod.deb && \
    apt-get update && \
    apt-get install -y dotnet-sdk-8.0 && \
    rm -rf /var/lib/apt/lists/*

# Install csharp-ls (C# language server)
RUN dotnet tool install --global csharp-ls

ENV PATH="${PATH}:/root/.dotnet/tools"

# Set up working directory
WORKDIR /workspace

# Default environment variables
ENV SERENA_CONTEXT=codex
ENV SERENA_PROJECT=/workspace

# Entrypoint runs the Serena MCP server with multi-language support
ENTRYPOINT ["serena", "start-mcp-server", "--context", "codex", "--project", "/workspace"]
