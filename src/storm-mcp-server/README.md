# Apache Storm MCP Server

A Model Context Protocol (MCP) server that provides expert knowledge and API interactions for Apache Storm clusters. This server enables AI assistants to help with Storm development, deployment, troubleshooting, and cluster management.

## Features

### Currently Implemented ✅

#### Storm Expertise Tools:
- `storm_help` - Get help with Storm concepts, APIs, and architecture
- `storm_best_practices` - Best practices for performance, reliability, security, and deployment
- `storm_troubleshoot` - Troubleshoot common Storm issues
- `generate_storm_topology` - Generate example topology code (Java, Python)

#### Storm Cluster Management (via Thrift):
- `connect_storm_cluster` - Connect to a real Storm cluster
- `disconnect_storm_cluster` - Disconnect from a cluster
- `get_cluster_info` - Get cluster information (supervisors, slots, topologies)
- `get_topology_info` - Get detailed topology information
- `kill_topology` - Kill a running topology
- `activate_topology` - Activate a deactivated topology
- `deactivate_topology` - Deactivate a running topology
- `rebalance_topology` - Rebalance topology workers and executors

### Coming Soon 🚧
- Topology submission with JAR upload
- Kubernetes deployment tools
- Advanced monitoring and metrics
- Configuration management

## Installation

```bash
# Create virtual environment
python3 -m venv storm-mcp-env
source storm-mcp-env/bin/activate

# Install dependencies
pip install -r requirements.txt
```

## Usage

### Quick Start

```bash
# Run the enhanced server (recommended)
./run_server.sh

# Run basic server (mock data only)
./run_server.sh --basic

# Test the server
./run_server.sh --test
```

### Manual Start

```bash
# Activate virtual environment
source storm-mcp-env/bin/activate

# Run enhanced server with Thrift support
python3 storm_mcp_enhanced.py

# Or run basic server (mock data only)
python3 storm_mcp_fixed.py
```

### Using with Claude Desktop

Add this configuration to your Claude Desktop settings:

```json
{
  "mcpServers": {
    "storm-mcp": {
      "command": "/path/to/storm-mcp-server/run_server.sh",
      "args": [],
      "env": {}
    }
  }
}
```

## Example Usage

### Get Storm Help
```python
result = await session.call_tool("storm_help", {"topic": "spouts"})
```

### Get Best Practices
```python
result = await session.call_tool("storm_best_practices", {"area": "performance"})
```

### Generate Topology Code
```python
result = await session.call_tool("generate_storm_topology", {
    "type": "wordcount",
    "language": "java"
})
```

### Connect to Real Storm Cluster
```python
# Connect to cluster
result = await session.call_tool("connect_storm_cluster", {
    "cluster_name": "prod",
    "nimbus_host": "storm-nimbus.example.com",
    "nimbus_port": 6627
})

# Get real cluster info
result = await session.call_tool("get_cluster_info", {"cluster_name": "prod"})

# Kill a topology
result = await session.call_tool("kill_topology", {
    "cluster_name": "prod",
    "topology_name": "old-topology",
    "wait_secs": 60
})
```

### Use Mock Data (No Cluster Required)
```python
result = await session.call_tool("get_cluster_info", {
    "cluster_name": "dev-cluster",
    "use_mock": True
})
```

## Architecture

The server is built using:
- **MCP 1.10.1** - Model Context Protocol for AI assistant integration
- **Apache Thrift** - For real Storm cluster communication
- **Python asyncio** - For async operations
- **Dual Mode** - Supports both mock data and real cluster connections

## Development

### Adding New Tools

1. Add the tool definition in `handle_list_tools()`
2. Implement the tool handler in `handle_call_tool()`
3. Add any helper functions needed

### Project Structure

```
storm-mcp-server/
├── storm_mcp_fixed.py          # Basic server (mock data only)
├── storm_mcp_enhanced.py       # Enhanced server with Thrift support
├── storm_thrift_client.py      # Thrift client for Storm clusters
├── test_*.py                   # Test clients
├── run_server.sh              # Convenience launcher script
├── requirements.txt           # Python dependencies
├── gen-py/storm/              # Generated Thrift bindings
│   ├── Nimbus.py             # Nimbus service client
│   └── ttypes.py             # Thrift type definitions
└── README.md                  # This file
```

### Implementation Status

See [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for detailed progress.

## License

Apache License 2.0