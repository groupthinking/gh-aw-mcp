#!/bin/bash

# Default values
HOST="${HOST:- 127.0.0.1}"
PORT="${PORT:-8000}"
CONFIG="${CONFIG}"
ENV_FILE="${ENV_FILE:-.env}"
MODE="${MODE:---routed}"

# Build the command
CMD="./flowguard-go"
FLAGS="$MODE --listen ${HOST}:${PORT}"

if [ -n "$ENV_FILE" ]; then
    FLAGS="$FLAGS --env $ENV_FILE"
fi

if [ -n "$CONFIG" ]; then
    FLAGS="$FLAGS --config $CONFIG"
else
    FLAGS="$FLAGS --config-stdin"
    CONFIG_JSON=$(cat <<EOF
{
    "mcpServers": {
        "github": {
            "type": "local",
            "container": "ghcr.io/github/github-mcp-server:latest",
            "env": {
                "GITHUB_PERSONAL_ACCESS_TOKEN": ""
            }
        },
        "fetch": {
            "type": "local",
            "container": "mcp/fetch"
        },
        "memory": {
            "type": "local",
            "container": "mcp/memory"
        }
    }
}
EOF
)
fi

echo "Starting FlowGuard Go server on port $PORT..."
echo "Command: $CMD $FLAGS"

# Execute the command
echo "$CONFIG_JSON" | $CMD $FLAGS
