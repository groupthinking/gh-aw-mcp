# Documentation

This directory contains documentation for the MCP Gateway project.

## Files

- **[mcp-gateway.md](mcp-gateway.md)** - Complete MCP Gateway Specification including configuration format, protocol behavior, authentication, and compliance requirements.

## Sync Process

The documentation in this directory is automatically synced from the [githubnext/gh-aw repository](https://github.com/githubnext/gh-aw/tree/main/docs/src/content/docs/reference) on a daily schedule.

- **Source**: https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md
- **Sync Script**: `scripts/sync-docs.sh`
- **Workflow**: `.github/workflows/sync-docs.yml`

### Manual Sync

To manually sync the documentation:

```bash
./scripts/sync-docs.sh
```

This will:
1. Fetch the latest documentation from the gh-aw repository
2. Remove Astro-specific frontmatter
3. Add a sync notice at the top
4. Save to `docs/mcp-gateway.md`

## Authoritative Source

The **official specification** is maintained in the [githubnext/gh-aw](https://github.com/githubnext/gh-aw) repository. The documentation here is a convenience copy for local reference. For compliance testing and official specification details, always refer to the source repository.
