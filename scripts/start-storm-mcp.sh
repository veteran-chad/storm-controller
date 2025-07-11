#!/bin/bash
cd "$(dirname "$0")/.."

# Activate virtual environment if it exists
if [ -d "src/storm-mcp-server/storm-mcp-env" ]; then
    source src/storm-mcp-server/storm-mcp-env/bin/activate
fi

# Set Python path
export PYTHONPATH="$PWD/src/storm-mcp-server:$PYTHONPATH"

# Start the MCP server
python src/storm-mcp-server/storm_mcp_server.py "$@"