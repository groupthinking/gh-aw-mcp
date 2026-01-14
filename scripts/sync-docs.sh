#!/bin/bash
set -euo pipefail

# Script to sync MCP Gateway documentation from githubnext/gh-aw repository
# This script fetches the latest mcp-gateway.md documentation and adapts it
# for local use in the gh-aw-mcpg repository.

DOCS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/docs"
SOURCE_URL="https://raw.githubusercontent.com/githubnext/gh-aw/main/docs/src/content/docs/reference/mcp-gateway.md"
TARGET_FILE="$DOCS_DIR/mcp-gateway.md"

echo "Syncing MCP Gateway documentation..."
echo "Source: $SOURCE_URL"
echo "Target: $TARGET_FILE"

# Create docs directory if it doesn't exist
mkdir -p "$DOCS_DIR"

# Fetch the documentation
echo "Fetching documentation..."
if ! curl -fsSL "$SOURCE_URL" -o "$TARGET_FILE.tmp"; then
    echo "Error: Failed to fetch documentation from $SOURCE_URL"
    exit 1
fi

# Remove Astro frontmatter (lines between --- markers at the start of the file)
echo "Adapting documentation format..."
awk '
BEGIN { in_frontmatter = 0; frontmatter_done = 0; }
/^---$/ && NR == 1 { in_frontmatter = 1; next; }
/^---$/ && in_frontmatter { in_frontmatter = 0; frontmatter_done = 1; next; }
!in_frontmatter && frontmatter_done { print; }
' "$TARGET_FILE.tmp" > "$TARGET_FILE.processed"

# Add sync notice at the top of the file
{
    echo ""
    echo "# MCP Gateway Specification"
    echo ""
    echo "> **Note**: This documentation is automatically synced from the [githubnext/gh-aw repository](https://github.com/githubnext/gh-aw/blob/main/docs/src/content/docs/reference/mcp-gateway.md) daily. The official specification is maintained in the gh-aw project and is the authoritative source for compliance testing."
    echo ""
    # Skip the original title line and add rest of content
    tail -n +3 "$TARGET_FILE.processed"
} > "$TARGET_FILE"

# Clean up temporary files
rm -f "$TARGET_FILE.tmp" "$TARGET_FILE.processed"

# Check if the file is not empty
if [ ! -s "$TARGET_FILE" ]; then
    echo "Error: Documentation file is empty after processing"
    exit 1
fi

echo "✓ Documentation synced successfully to $TARGET_FILE"
echo "✓ File size: $(wc -c < "$TARGET_FILE") bytes"
echo "✓ Line count: $(wc -l < "$TARGET_FILE") lines"
