# Using the Unified aw-serena Container

This guide explains how to use the unified `aw-serena` container and configure it for your specific needs.

## Overview

The unified `aw-serena` container includes support for all 6 programming languages:
- Go
- TypeScript (JavaScript/Node.js)
- Python
- Java
- Rust
- C#

**Important:** While all language runtimes are installed, Serena itself determines which languages to activate based on your project structure and configuration.

## Basic Usage

### Quick Start

Run the container with your project mounted:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  ghcr.io/githubnext/aw-serena:latest
```

This will:
1. Mount your current directory to `/workspace`
2. Start Serena with default context (`codex`)
3. Auto-detect languages based on project files

### With MCP Gateway

Configure in your MCP Gateway config:

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

## Enabling Specific Languages

Serena automatically detects which languages are used in your project by scanning for:

- **Go:** `go.mod`, `*.go` files
- **TypeScript:** `tsconfig.json`, `package.json`, `*.ts` files
- **Python:** `requirements.txt`, `pyproject.toml`, `*.py` files
- **Java:** `pom.xml`, `build.gradle`, `*.java` files
- **Rust:** `Cargo.toml`, `*.rs` files
- **C#:** `*.csproj`, `*.sln`, `*.cs` files

### Method 1: Auto-Detection (Recommended)

Simply mount your project directory. Serena will automatically detect and enable the appropriate languages:

```bash
docker run -it --rm \
  -v /path/to/your/go-project:/workspace \
  ghcr.io/githubnext/aw-serena:latest
```

### Method 2: Explicit Language Configuration

If Serena supports explicit language configuration (check Serena documentation), you can pass additional arguments:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  ghcr.io/githubnext/aw-serena:latest \
  serena start-mcp-server \
    --context codex \
    --project /workspace \
    --languages go,typescript
```

### Method 3: Environment Variables

Some language servers can be controlled via environment variables:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  -e SERENA_CONTEXT=codex \
  -e GOPLS_ENABLED=true \
  -e PYRIGHT_ENABLED=false \
  ghcr.io/githubnext/aw-serena:latest
```

**Note:** Check Serena's official documentation for supported environment variables.

## Common Use Cases

### Single Language Project (Go)

For a Go-only project, the unified container will only activate Go support:

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
        "SERENA_PROJECT": "/workspace"
      }
    }
  }
}
```

**Performance Note:** Even though unused languages are installed, they won't impact performance if not activated.

### Multi-Language Monorepo

For a project with multiple languages (e.g., Go backend + TypeScript frontend):

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

Serena will detect:
- `/workspace/backend/go.mod` → Enable Go
- `/workspace/frontend/package.json` → Enable TypeScript

### Subproject Focus

To focus on a specific subdirectory:

```bash
docker run -it --rm \
  -v $(pwd)/backend:/workspace \
  ghcr.io/githubnext/aw-serena:latest
```

This limits Serena's scope to just the `backend` directory.

## Performance Optimization

### When to Use Unified vs Language-Specific

**Use unified `aw-serena` when:**
- ✅ Project uses 2+ languages
- ✅ Uncertain which languages will be needed
- ✅ Flexibility is more important than size
- ✅ Disk space ~2-4GB is acceptable

**Use language-specific (e.g., `serena-go`) when:**
- ✅ Project uses only one language
- ✅ Need minimal image size (~200-600MB)
- ✅ Fast startup time is critical
- ✅ Production deployment with strict resource limits

### Resource Requirements

**Unified Container:**
- Disk: ~2-4GB (image size)
- RAM: ~512MB-2GB (depends on active languages)
- CPU: Varies by language server usage

**Language-Specific Containers:**
- Disk: ~200-600MB per image
- RAM: ~256MB-1GB (single language overhead)
- CPU: Lower baseline usage

## Advanced Configuration

### Custom Entrypoint

Override the default entrypoint for custom configuration:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  --entrypoint /bin/bash \
  ghcr.io/githubnext/aw-serena:latest
```

Then manually start Serena with specific options:

```bash
serena start-mcp-server \
  --context codex \
  --project /workspace \
  --verbose \
  --log-level debug
```

### Multiple Workspace Directories

Mount multiple directories if needed:

```bash
docker run -it --rm \
  -v $(pwd)/src:/workspace/src \
  -v $(pwd)/tests:/workspace/tests \
  -v $(pwd)/docs:/workspace/docs \
  ghcr.io/githubnext/aw-serena:latest
```

### Language Server Configuration

Each language server can be configured. Create configuration files in your project:

**Go (gopls):** `.gopls.json` or `gopls.yaml`
**TypeScript:** `tsconfig.json`
**Python:** `pyproject.toml` with `[tool.pylsp]`
**Rust:** `.rust-analyzer.json`

Example `.gopls.json`:

```json
{
  "analyses": {
    "unusedparams": true,
    "shadow": true
  },
  "staticcheck": true
}
```

## Troubleshooting

### Language Not Detected

**Problem:** Serena doesn't recognize your language.

**Solutions:**
1. Ensure language-specific files exist (e.g., `go.mod` for Go)
2. Check file permissions (files must be readable)
3. Try explicit language specification (if supported)
4. Check Serena logs for detection issues

### Out of Memory

**Problem:** Container crashes with OOM errors.

**Solutions:**
1. Increase Docker memory limit:
   ```bash
   docker run -it --rm -m 4g \
     -v $(pwd):/workspace \
     ghcr.io/githubnext/aw-serena:latest
   ```
2. Use language-specific container instead
3. Limit workspace scope to smaller subdirectories

### Slow Startup

**Problem:** Container takes long to start.

**Solutions:**
1. Use language-specific container for single-language projects
2. Reduce mounted directory size
3. Enable Docker layer caching
4. Use SSD for Docker storage

### Language Server Not Working

**Problem:** Specific language features aren't working.

**Solutions:**
1. Verify language runtime is available:
   ```bash
   docker run -it --rm ghcr.io/githubnext/aw-serena:latest which go
   docker run -it --rm ghcr.io/githubnext/aw-serena:latest which node
   docker run -it --rm ghcr.io/githubnext/aw-serena:latest which python3
   ```
2. Check language server installation:
   ```bash
   docker run -it --rm ghcr.io/githubnext/aw-serena:latest gopls version
   docker run -it --rm ghcr.io/githubnext/aw-serena:latest typescript-language-server --version
   ```
3. Review Serena logs for language server errors

## Migration Guide

### From Language-Specific to Unified

**Before:**
```json
{
  "mcpServers": {
    "serena-go": {
      "container": "ghcr.io/githubnext/serena-go:latest",
      ...
    },
    "serena-typescript": {
      "container": "ghcr.io/githubnext/serena-typescript:latest",
      ...
    }
  }
}
```

**After:**
```json
{
  "mcpServers": {
    "serena": {
      "container": "ghcr.io/githubnext/aw-serena:latest",
      ...
    }
  }
}
```

**Benefits:**
- Single server instance
- Simplified configuration
- Better for multi-language projects

**Trade-offs:**
- Larger image download
- More disk space required

### From Unified to Language-Specific

**Before:**
```json
{
  "mcpServers": {
    "serena": {
      "container": "ghcr.io/githubnext/aw-serena:latest",
      ...
    }
  }
}
```

**After:**
```json
{
  "mcpServers": {
    "serena-go": {
      "container": "ghcr.io/githubnext/serena-go:latest",
      ...
    }
  }
}
```

**Benefits:**
- Smaller image size
- Faster startup
- Better security posture

**Trade-offs:**
- Need to choose correct image
- Multiple images for multi-language projects

## Best Practices

1. **Start with unified for prototyping** - Use `aw-serena` during development
2. **Optimize for production** - Switch to language-specific for deployment
3. **Pin versions** - Use specific tags instead of `latest` in production
4. **Monitor resources** - Track memory and disk usage
5. **Update regularly** - Pull latest images for security updates

## Examples

### Example 1: Python Data Science Project

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  ghcr.io/githubnext/aw-serena:latest
```

Auto-detects Python from `requirements.txt` and `*.py` files.

### Example 2: Full-Stack Monorepo

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  ghcr.io/githubnext/aw-serena:latest
```

Auto-detects:
- Go backend (`/api/go.mod`)
- TypeScript frontend (`/web/package.json`)
- Python scripts (`/scripts/*.py`)

### Example 3: Java Microservice

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e SERENA_PROJECT=/workspace \
  -e JAVA_HOME=/usr/lib/jvm/temurin-21-jdk-amd64 \
  ghcr.io/githubnext/aw-serena:latest
```

Auto-detects Java from `pom.xml` or `build.gradle`.

## Further Reading

- [Serena Documentation](https://github.com/oraios/serena)
- [MCP Gateway Configuration](../README.md)
- [Language Server Protocol](https://microsoft.github.io/language-server-protocol/)
- [Container Best Practices](https://docs.docker.com/develop/dev-best-practices/)

## Getting Help

If you encounter issues:

1. Check Serena logs for errors
2. Verify language detection
3. Review this troubleshooting guide
4. Consult Serena documentation
5. Open an issue on GitHub
