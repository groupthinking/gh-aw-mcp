# Serena MCP Server Containers

This directory contains Dockerfiles for building Serena MCP server containers with language-specific support.

## Overview

Serena is an AI-powered code intelligence tool that provides language service integration through the Model Context Protocol (MCP). These containers package Serena with language-specific tooling for seamless integration with GitHub Agentic Workflows.

## Available Containers

| Language   | Container Image | Dockerfile |
|------------|----------------|------------|
| Go         | `ghcr.io/githubnext/serena-go:latest` | `Dockerfile-go` |
| TypeScript | `ghcr.io/githubnext/serena-typescript:latest` | `Dockerfile-typescript` |
| Python     | `ghcr.io/githubnext/serena-python:latest` | `Dockerfile-python` |
| Java       | `ghcr.io/githubnext/serena-java:latest` | `Dockerfile-java` |
| Rust       | `ghcr.io/githubnext/serena-rust:latest` | `Dockerfile-rust` |
| C#         | `ghcr.io/githubnext/serena-csharp:latest` | `Dockerfile-csharp` |

## Usage

### With MCP Gateway

Configure Serena in your MCP Gateway config:

```json
{
  "mcpServers": {
    "serena-go": {
      "type": "stdio",
      "container": "ghcr.io/githubnext/serena-go:latest",
      "mounts": [
        "${PWD}:/workspace:rw"
      ],
      "env": {
        "SERENA_PROJECT": "/workspace",
        "SERENA_CONTEXT": "codex"
      }
    }
  }
}
```

### Standalone Usage

Run Serena directly:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  -e SERENA_CONTEXT=codex \
  ghcr.io/githubnext/serena-go:latest
```

## Environment Variables

- `SERENA_PROJECT`: Path to the project directory (default: `/workspace`)
- `SERENA_CONTEXT`: Serena context mode (default: `codex`)

## Language-Specific Tools

Each container includes:

### Go
- Go 1.23
- gopls (Go language server)
- Git, Python, pip

### TypeScript
- Node.js 22
- TypeScript compiler
- typescript-language-server
- Git, Python, pip

### Python
- Python 3.12
- python-lsp-server
- Git

### Java
- Eclipse Temurin 21
- Eclipse JDT Language Server
- Git, Python, pip

### Rust
- Rust 1.75
- rust-analyzer
- Git, Python, pip

### C#
- .NET SDK 8.0
- csharp-ls (C# language server)
- Git, Python, pip

## Building Locally

Build a specific language container:

```bash
docker build -f serena/Dockerfile-go -t serena-go:local .
```

Build all containers:

```bash
for lang in go typescript python java rust csharp; do
  docker build -f serena/Dockerfile-$lang -t serena-$lang:local .
done
```

## CI/CD

Containers are automatically built and pushed to GitHub Container Registry via the `.github/workflows/serena-containers.yml` workflow when:

- Commits are pushed to the `main` branch
- Tags starting with `serena-v*` are created
- The workflow is manually triggered

## Source

Serena is developed by the Oraios team: https://github.com/oraios/serena
