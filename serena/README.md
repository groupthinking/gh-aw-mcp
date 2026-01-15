# Serena MCP Server Containers

This directory contains Dockerfiles for building Serena MCP server containers with language support.

## Overview

Serena is an AI-powered code intelligence tool that provides language service integration through the Model Context Protocol (MCP). These containers package Serena with language-specific tooling for seamless integration with GitHub Agentic Workflows.

## Available Container Options

### Option 1: Unified Multi-Language Container (Recommended for Multi-Language Projects)

**Container:** `ghcr.io/githubnext/aw-serena:latest`  
**Dockerfile:** `Dockerfile`

This unified container includes support for **all languages** in a single image.

**✅ Advantages:**
- Single image to manage and deploy
- Works with all supported languages out-of-the-box
- No need to choose a specific language image
- Ideal for polyglot projects

**❌ Drawbacks:**
- **Large size:** ~2-4GB (vs ~200-500MB for language-specific)
- **Slower startup:** More dependencies to initialize
- **Wasteful:** Most projects only use 1-2 languages
- **Security:** Larger attack surface with all runtimes
- **Build time:** Much longer to build
- **Poor caching:** Updates require rebuilding entire image

**Supported Languages:** Go, TypeScript, Python, Java, Rust, C#

### Option 2: Language-Specific Containers (Recommended for Single-Language Projects)

Optimized containers for individual languages:

| Language   | Container Image | Size | Dockerfile |
|------------|----------------|------|------------|
| Go         | `ghcr.io/githubnext/serena-go:latest` | ~300MB | `Dockerfile-go` |
| TypeScript | `ghcr.io/githubnext/serena-typescript:latest` | ~250MB | `Dockerfile-typescript` |
| Python     | `ghcr.io/githubnext/serena-python:latest` | ~200MB | `Dockerfile-python` |
| Java       | `ghcr.io/githubnext/serena-java:latest` | ~500MB | `Dockerfile-java` |
| Rust       | `ghcr.io/githubnext/serena-rust:latest` | ~600MB | `Dockerfile-rust` |
| C#         | `ghcr.io/githubnext/serena-csharp:latest` | ~400MB | `Dockerfile-csharp` |

**✅ Advantages:**
- **Small size:** Optimized for specific language
- **Fast startup:** Minimal dependencies
- **Secure:** Minimal attack surface
- **Efficient:** Only what you need
- **Better caching:** Layer reuse is more effective

**❌ Drawbacks:**
- Need to choose the correct image for your project
- Multiple images to manage for polyglot projects

## Which Should You Use?

**Choose the unified container (`aw-serena`) if:**
- Your project uses multiple programming languages
- You want maximum flexibility without worrying about language selection
- Image size is not a critical concern (~2-4GB is acceptable)
- You're prototyping or experimenting

**Choose a language-specific container if:**
- Your project primarily uses one language
- You need fast startup times
- You want minimal image size
- Security/minimal attack surface is important
- You're deploying to production

## Usage

### With MCP Gateway (Unified Container)

Configure the unified Serena container in your MCP Gateway config:

```json
{
  "mcpServers": {
    "serena": {
      "type": "stdio",
      "container": "ghcr.io/githubnext/aw-serena:latest",
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

### With MCP Gateway (Language-Specific)

Configure a language-specific Serena container:

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

### Standalone Usage (Unified Container)

Run Serena with all language support:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  -e SERENA_CONTEXT=codex \
  ghcr.io/githubnext/aw-serena:latest
```

### Standalone Usage (Language-Specific)

Run Serena with Go support only:

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

### Unified Container Includes

All languages and their tools in one image:

- **Go 1.23** - gopls (Go language server)
- **Node.js 22** - TypeScript compiler, typescript-language-server
- **Python 3.12** - python-lsp-server
- **Java 21** (Eclipse Temurin) - Eclipse JDT Language Server
- **Rust 1.75** - rust-analyzer
- **.NET SDK 8.0** - csharp-ls (C# language server)
- **Common tools:** Git, curl, wget, build-essential

### Language-Specific Containers Include

Each container includes only what's needed for that language:

#### Go
- Go 1.23
- gopls (Go language server)
- Git, Python, pip

#### TypeScript
- Node.js 22
- TypeScript compiler
- typescript-language-server
- Git, Python, pip

#### Python
- Python 3.12
- python-lsp-server
- Git

#### Java
- Eclipse Temurin 21
- Eclipse JDT Language Server
- Git, Python, pip

#### Rust
- Rust 1.75
- rust-analyzer
- Git, Python, pip

#### C#
- .NET SDK 8.0
- csharp-ls (C# language server)
- Git, Python, pip

## Building Locally

### Build the unified container:

```bash
docker build -f serena/Dockerfile -t aw-serena:local .
```

### Build a specific language container:

```bash
docker build -f serena/Dockerfile-go -t serena-go:local .
```

### Build all containers:

```bash
# Unified container
docker build -f serena/Dockerfile -t aw-serena:local .

# Language-specific containers
for lang in go typescript python java rust csharp; do
  docker build -f serena/Dockerfile-$lang -t serena-$lang:local .
done
```

## CI/CD

Containers are automatically built and pushed to GitHub Container Registry via the `.github/workflows/serena-containers.yml` workflow when:

- Commits are pushed to the `main` branch
- Tags starting with `serena-v*` are created
- The workflow is manually triggered

The workflow builds:
1. **Unified container:** `ghcr.io/githubnext/aw-serena:latest`
2. **Language-specific containers:** All 6 individual language images

## Testing

A comprehensive test suite is available for verifying Serena containers:

### MCP Test Client

Located in `test/mcp-client/`, the test client provides:
- Python MCP client library for testing stdio-based MCP servers
- Pytest test suite for Serena containers
- Docker test client container with all dependencies
- Integration with Go test suite

**Quick test:**

```bash
# Test a container with Python client
cd test/mcp-client
python3 mcp_client.py aw-serena:local /path/to/project

# Run full pytest suite
pytest -v test_serena.py

# Run in test client container
docker build -f serena/Dockerfile.test-client -t mcp-test-client .
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd):/workspace \
  -e USE_LOCAL_IMAGES=1 \
  mcp-test-client \
  /workspace/test/mcp-client/test_serena.py
```

**Go integration tests:**

```bash
# Run Serena integration tests
go test -v ./test/integration -run TestSerena

# Run all integration tests
make test-integration
```

For detailed testing documentation, see [test/mcp-client/README.md](../test/mcp-client/README.md).

## Image Size Comparison

| Container Type | Approximate Size | Use Case |
|---------------|------------------|----------|
| `aw-serena` (unified) | ~2-4GB | Multi-language projects |
| `serena-go` | ~300MB | Go-only projects |
| `serena-typescript` | ~250MB | TypeScript-only projects |
| `serena-python` | ~200MB | Python-only projects |
| `serena-java` | ~500MB | Java-only projects |
| `serena-rust` | ~600MB | Rust-only projects |
| `serena-csharp` | ~400MB | C#-only projects |

## Source

Serena is developed by the Oraios team: https://github.com/oraios/serena
