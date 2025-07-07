#!/usr/bin/env python3
"""
Apache Storm Expert MCP Server - Fixed Version

This MCP server provides comprehensive Apache Storm expertise including:
- Documentation and code knowledge from Storm project
- Thrift API interactions with Storm clusters
- Storm topology management and monitoring
- Best practices and troubleshooting guidance
"""

import asyncio
import json
import logging
from typing import Any, Dict, List, Optional, Union, Sequence

# MCP imports
from mcp.server import Server
from mcp.server.models import InitializationOptions
import mcp.server.stdio
import mcp.types as types

# Thrift imports for Storm API
try:
    from thrift.transport import TSocket, TTransport
    from thrift.protocol import TBinaryProtocol
    from thrift.transport.TTransport import TFramedTransport
    THRIFT_AVAILABLE = True
except ImportError:
    THRIFT_AVAILABLE = False
    print("Warning: Thrift library not available. Install with: pip install thrift")

# Storm thrift client imports
try:
    import sys
    import os
    # Add gen-py to Python path
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'gen-py'))
    from storm.Nimbus import Client as NimbusClient
    from storm import ttypes
    STORM_THRIFT_AVAILABLE = True
except ImportError as e:
    STORM_THRIFT_AVAILABLE = False
    print(f"Warning: Storm thrift bindings not available: {e}")

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("storm-mcp-server")


class StormThriftClient:
    """
    Handles thrift connections to Storm Nimbus for cluster operations
    """
    
    def __init__(self, nimbus_host: str = "localhost", nimbus_port: int = 6627):
        self.nimbus_host = nimbus_host
        self.nimbus_port = nimbus_port
        self.client = None
        self.transport = None
        
    def connect(self) -> bool:
        """Establish connection to Storm Nimbus"""
        if not THRIFT_AVAILABLE or not STORM_THRIFT_AVAILABLE:
            logger.error("Thrift libraries not available")
            return False
            
        try:
            # Create transport and protocol
            socket = TSocket.TSocket(self.nimbus_host, self.nimbus_port)
            self.transport = TFramedTransport(socket)
            protocol = TBinaryProtocol.TBinaryProtocol(self.transport)
            self.client = NimbusClient(protocol)
            
            # Open connection
            self.transport.open()
            logger.info(f"Connected to Storm Nimbus at {self.nimbus_host}:{self.nimbus_port}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to connect to Nimbus: {e}")
            return False
    
    def disconnect(self):
        """Close connection to Storm Nimbus"""
        if self.transport:
            self.transport.close()
            logger.info("Disconnected from Storm Nimbus")
    
    def get_cluster_info(self) -> Dict[str, Any]:
        """Get Storm cluster information"""
        if not self.client:
            return {"error": "Not connected to Nimbus"}
            
        try:
            cluster_info = self.client.getClusterInfo()
            return {
                "supervisors": len(cluster_info.supervisors),
                "nimbus_uptime": cluster_info.nimbus_uptime_secs,
                "topologies": [
                    {
                        "name": topo.name,
                        "id": topo.id,
                        "status": topo.status,
                        "num_workers": topo.num_workers,
                        "num_executors": topo.num_executors,
                        "num_tasks": topo.num_tasks,
                        "uptime_secs": topo.uptime_secs
                    }
                    for topo in cluster_info.topologies
                ]
            }
        except Exception as e:
            return {"error": f"Failed to get cluster info: {e}"}


class StormExpertServer:
    """
    Main MCP server class providing Storm expertise
    """
    
    def __init__(self):
        self.server = Server("storm-expert")
        self.storm_client = None
        self.setup_handlers()
    
    def setup_handlers(self):
        """Set up MCP server handlers"""
        
        @self.server.list_tools()
        async def handle_list_tools() -> List[types.Tool]:
            """List available Storm tools"""
            return [
                types.Tool(
                    name="connect_storm_cluster",
                    description="Connect to a Storm cluster via Thrift API",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "nimbus_host": {
                                "type": "string",
                                "description": "Nimbus host address",
                                "default": "localhost"
                            },
                            "nimbus_port": {
                                "type": "integer",
                                "description": "Nimbus port",
                                "default": 6627
                            }
                        }
                    }
                ),
                types.Tool(
                    name="get_cluster_info",
                    description="Get Storm cluster information including supervisors and topologies",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                types.Tool(
                    name="storm_troubleshoot",
                    description="Provide troubleshooting guidance for Storm issues",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "issue_type": {
                                "type": "string",
                                "description": "Type of issue (performance, connectivity, topology, etc.)"
                            },
                            "symptoms": {
                                "type": "string",
                                "description": "Description of observed symptoms"
                            }
                        },
                        "required": ["issue_type", "symptoms"]
                    }
                ),
                types.Tool(
                    name="storm_best_practices",
                    description="Get Storm development and deployment best practices",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "category": {
                                "type": "string",
                                "description": "Category of best practices (topology_design, performance, reliability, etc.)"
                            }
                        },
                        "required": ["category"]
                    }
                ),
                types.Tool(
                    name="generate_storm_topology",
                    description="Generate sample Storm topology code based on requirements",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "topology_type": {
                                "type": "string",
                                "description": "Type of topology (word_count, real_time_analytics, stream_processing, etc.)"
                            },
                            "language": {
                                "type": "string",
                                "description": "Programming language (Java, Python, Scala)",
                                "default": "Java"
                            },
                            "requirements": {
                                "type": "string",
                                "description": "Specific requirements for the topology"
                            }
                        },
                        "required": ["topology_type"]
                    }
                )
            ]
        
        @self.server.call_tool()
        async def handle_call_tool(name: str, arguments: Dict[str, Any]) -> List[types.TextContent]:
            """Handle tool calls"""
            
            if name == "connect_storm_cluster":
                return await self._connect_storm_cluster(arguments)
            elif name == "get_cluster_info":
                return await self._get_cluster_info()
            elif name == "storm_troubleshoot":
                return await self._storm_troubleshoot(arguments)
            elif name == "storm_best_practices":
                return await self._storm_best_practices(arguments)
            elif name == "generate_storm_topology":
                return await self._generate_storm_topology(arguments)
            else:
                raise ValueError(f"Unknown tool: {name}")
    
    async def _connect_storm_cluster(self, arguments: Dict[str, Any]) -> List[types.TextContent]:
        """Connect to Storm cluster"""
        nimbus_host = arguments.get("nimbus_host", "localhost")
        nimbus_port = arguments.get("nimbus_port", 6627)
        
        self.storm_client = StormThriftClient(nimbus_host, nimbus_port)
        
        if self.storm_client.connect():
            return [types.TextContent(
                type="text",
                text=f"✅ Successfully connected to Storm cluster at {nimbus_host}:{nimbus_port}"
            )]
        else:
            return [types.TextContent(
                type="text",
                text=f"❌ Failed to connect to Storm cluster at {nimbus_host}:{nimbus_port}. Make sure Storm Nimbus is running and thrift bindings are available."
            )]
    
    async def _get_cluster_info(self) -> List[types.TextContent]:
        """Get cluster information"""
        if not self.storm_client:
            return [types.TextContent(
                type="text",
                text="❌ Not connected to Storm cluster. Use connect_storm_cluster first."
            )]
        
        cluster_info = self.storm_client.get_cluster_info()
        
        if "error" in cluster_info:
            return [types.TextContent(
                type="text",
                text=f"❌ Error getting cluster info: {cluster_info['error']}"
            )]
        
        # Format cluster information
        result = f"""🌩️ **Storm Cluster Information**

**Cluster Overview:**
- Supervisors: {cluster_info['supervisors']}
- Nimbus Uptime: {cluster_info['nimbus_uptime']} seconds

**Active Topologies ({len(cluster_info['topologies'])}):**
"""
        
        for topo in cluster_info['topologies']:
            result += f"""
- **{topo['name']}** ({topo['id']})
  - Status: {topo['status']}
  - Workers: {topo['num_workers']}
  - Executors: {topo['num_executors']}
  - Tasks: {topo['num_tasks']}
  - Uptime: {topo['uptime_secs']} seconds
"""
        
        return [types.TextContent(type="text", text=result)]
    
    async def _storm_troubleshoot(self, arguments: Dict[str, Any]) -> List[types.TextContent]:
        """Provide troubleshooting guidance"""
        issue_type = arguments["issue_type"].lower()
        symptoms = arguments["symptoms"]
        
        troubleshooting_guide = {
            "performance": {
                "title": "🚀 Storm Performance Issues",
                "common_causes": [
                    "Insufficient parallelism configuration",
                    "Bolt processing slower than spout emission rate",
                    "Network bottlenecks between nodes",
                    "Garbage collection issues",
                    "Serialization overhead"
                ],
                "solutions": [
                    "Increase parallelism hint for slow components",
                    "Use fields grouping to ensure load balancing",
                    "Monitor and tune JVM heap settings",
                    "Optimize serialization with Kryo",
                    "Use multiple workers per topology",
                    "Check for data skew in groupings"
                ]
            },
            "connectivity": {
                "title": "🔌 Storm Connectivity Issues",
                "common_causes": [
                    "ZooKeeper connection problems",
                    "Network firewall blocking ports",
                    "Nimbus not accessible from workers",
                    "Supervisor connection issues"
                ],
                "solutions": [
                    "Check ZooKeeper cluster status",
                    "Verify storm.zookeeper.servers configuration",
                    "Ensure ports 6627 (Nimbus) and 6700-6703 (Workers) are open",
                    "Check nimbus.host configuration on supervisors",
                    "Verify network connectivity between nodes"
                ]
            },
            "topology": {
                "title": "⚡ Storm Topology Issues",
                "common_causes": [
                    "Unbalanced topology parallelism",
                    "Tuple processing failures",
                    "Memory leaks in bolts",
                    "Acker bottlenecks"
                ],
                "solutions": [
                    "Review and adjust parallelism configuration",
                    "Implement proper error handling in bolts",
                    "Use tuple anchoring and acking correctly",
                    "Monitor topology metrics via Storm UI",
                    "Consider disabling acking for high-throughput scenarios"
                ]
            }
        }
        
        guide = troubleshooting_guide.get(issue_type, {
            "title": "🔍 General Storm Troubleshooting",
            "common_causes": ["Various system and configuration issues"],
            "solutions": ["Check Storm UI for metrics", "Review logs for error messages", "Verify cluster configuration"]
        })
        
        result = f"""{guide['title']}

**Symptoms:** {symptoms}

**Common Causes:**
"""
        for cause in guide['common_causes']:
            result += f"• {cause}\n"
        
        result += "\n**Recommended Solutions:**\n"
        for solution in guide['solutions']:
            result += f"• {solution}\n"
        
        result += """
**Additional Resources:**
• Storm UI: http://nimbus-host:8080
• Check logs: storm logs
• Monitor with JMX/Ganglia
• Use storm rebalance for runtime adjustments
"""
        
        return [types.TextContent(type="text", text=result)]
    
    async def _storm_best_practices(self, arguments: Dict[str, Any]) -> List[types.TextContent]:
        """Provide best practices guidance"""
        category = arguments["category"].lower()
        
        best_practices = {
            "topology_design": {
                "title": "🏗️ Storm Topology Design Best Practices",
                "practices": [
                    "Keep bolt processing logic simple and fast",
                    "Use appropriate stream groupings (fields, shuffle, all)",
                    "Design for idempotency when possible",
                    "Minimize state in bolts",
                    "Use Trident for exactly-once processing needs",
                    "Plan parallelism based on expected throughput",
                    "Consider data locality in grouping decisions"
                ]
            },
            "performance": {
                "title": "⚡ Storm Performance Best Practices", 
                "practices": [
                    "Tune topology parallelism carefully",
                    "Use Kryo serialization for better performance",
                    "Batch operations where possible",
                    "Avoid blocking operations in bolts",
                    "Use multiple workers per topology",
                    "Monitor and tune JVM settings",
                    "Consider disabling acking for high-throughput scenarios",
                    "Use local mode for development and testing"
                ]
            },
            "reliability": {
                "title": "🛡️ Storm Reliability Best Practices",
                "practices": [
                    "Implement proper tuple anchoring and acking",
                    "Handle failures gracefully with try-catch blocks",
                    "Use replay mechanisms for critical data",
                    "Monitor topology health with metrics",
                    "Set appropriate timeouts for processing",
                    "Use transactional topologies for guaranteed processing",
                    "Implement circuit breakers for external dependencies",
                    "Plan for node failures and recovery"
                ]
            },
            "monitoring": {
                "title": "📊 Storm Monitoring Best Practices",
                "practices": [
                    "Use Storm UI for real-time monitoring",
                    "Set up JMX monitoring for detailed metrics",
                    "Monitor tuple flow rates and latencies",
                    "Track error rates and failed tuples",
                    "Use external monitoring tools (Ganglia, Graphite)",
                    "Set up alerting for topology failures",
                    "Log important events and errors",
                    "Monitor resource utilization (CPU, memory, network)"
                ]
            }
        }
        
        practices = best_practices.get(category, {
            "title": "📋 General Storm Best Practices",
            "practices": [
                "Follow the official Storm documentation",
                "Test topologies thoroughly in local mode",
                "Plan for scalability from the beginning",
                "Keep configurations in version control"
            ]
        })
        
        result = f"{practices['title']}\n\n"
        for i, practice in enumerate(practices['practices'], 1):
            result += f"{i}. {practice}\n"
        
        return [types.TextContent(type="text", text=result)]
    
    async def _generate_storm_topology(self, arguments: Dict[str, Any]) -> List[types.TextContent]:
        """Generate sample Storm topology code"""
        topology_type = arguments["topology_type"].lower()
        language = arguments.get("language", "Java").lower()
        requirements = arguments.get("requirements", "")
        
        if language == "java" and topology_type == "word_count":
            code = '''// Word Count Storm Topology Example
import org.apache.storm.Config;
import org.apache.storm.LocalCluster;
import org.apache.storm.StormSubmitter;
import org.apache.storm.spout.SpoutOutputCollector;
import org.apache.storm.task.TopologyContext;
import org.apache.storm.topology.BasicOutputCollector;
import org.apache.storm.topology.OutputFieldsDeclarer;
import org.apache.storm.topology.TopologyBuilder;
import org.apache.storm.topology.base.BaseBasicBolt;
import org.apache.storm.topology.base.BaseRichSpout;
import org.apache.storm.tuple.Fields;
import org.apache.storm.tuple.Tuple;
import org.apache.storm.tuple.Values;

import java.util.HashMap;
import java.util.Map;
import java.util.StringTokenizer;

public class WordCountTopology {
    
    public static class SentenceSpout extends BaseRichSpout {
        private SpoutOutputCollector collector;
        private String[] sentences = {
            "the cow jumped over the moon",
            "an apple a day keeps the doctor away",
            "four score and seven years ago"
        };
        private int index = 0;

        @Override
        public void open(Map conf, TopologyContext context, 
                        SpoutOutputCollector collector) {
            this.collector = collector;
        }

        @Override
        public void nextTuple() {
            this.collector.emit(new Values(sentences[index]));
            index++;
            if (index >= sentences.length) {
                index = 0;
            }
            try { Thread.sleep(100); } catch (InterruptedException e) {}
        }

        @Override
        public void declareOutputFields(OutputFieldsDeclarer declarer) {
            declarer.declare(new Fields("sentence"));
        }
    }

    public static class SplitSentenceBolt extends BaseBasicBolt {
        @Override
        public void execute(Tuple tuple, BasicOutputCollector collector) {
            String sentence = tuple.getString(0);
            StringTokenizer tokenizer = new StringTokenizer(sentence);
            while (tokenizer.hasMoreTokens()) {
                collector.emit(new Values(tokenizer.nextToken()));
            }
        }

        @Override
        public void declareOutputFields(OutputFieldsDeclarer declarer) {
            declarer.declare(new Fields("word"));
        }
    }

    public static class WordCountBolt extends BaseBasicBolt {
        private Map<String, Integer> counts = new HashMap<>();

        @Override
        public void execute(Tuple tuple, BasicOutputCollector collector) {
            String word = tuple.getString(0);
            Integer count = counts.get(word);
            if (count == null) count = 0;
            count++;
            counts.put(word, count);
            collector.emit(new Values(word, count));
        }

        @Override
        public void declareOutputFields(OutputFieldsDeclarer declarer) {
            declarer.declare(new Fields("word", "count"));
        }
    }

    public static void main(String[] args) throws Exception {
        TopologyBuilder builder = new TopologyBuilder();
        
        builder.setSpout("sentence-spout", new SentenceSpout(), 1);
        builder.setBolt("split-bolt", new SplitSentenceBolt(), 2)
               .shuffleGrouping("sentence-spout");
        builder.setBolt("count-bolt", new WordCountBolt(), 2)
               .fieldsGrouping("split-bolt", new Fields("word"));

        Config config = new Config();
        config.setDebug(true);

        if (args != null && args.length > 0) {
            config.setNumWorkers(3);
            StormSubmitter.submitTopology(args[0], config, 
                                        builder.createTopology());
        } else {
            LocalCluster cluster = new LocalCluster();
            cluster.submitTopology("word-count", config, 
                                 builder.createTopology());
            Thread.sleep(10000);
            cluster.shutdown();
        }
    }
}'''
        else:
            # Generic template
            code = f'''// Generic {topology_type.title()} Storm Topology Template ({language.title()})

/**
 * Requirements: {requirements}
 *
 * This is a template for a {topology_type} topology.
 * Customize the spouts and bolts according to your specific needs.
 */

public class {topology_type.title().replace('_', '')}Topology {{
    
    // Define your spout class
    public static class DataSpout extends BaseRichSpout {{
        // Implement spout logic here
    }}
    
    // Define your bolt classes
    public static class ProcessingBolt extends BaseBasicBolt {{
        // Implement bolt logic here
    }}
    
    // Main topology definition
    public static void main(String[] args) throws Exception {{
        TopologyBuilder builder = new TopologyBuilder();
        
        // Configure topology
        builder.setSpout("data-spout", new DataSpout(), 1);
        builder.setBolt("processing-bolt", new ProcessingBolt(), 2)
               .shuffleGrouping("data-spout");
        
        // Submit topology
        Config config = new Config();
        if (args != null && args.length > 0) {{
            StormSubmitter.submitTopology(args[0], config, builder.createTopology());
        }} else {{
            LocalCluster cluster = new LocalCluster();
            cluster.submitTopology("{topology_type}", config, builder.createTopology());
        }}
    }}
}}'''
        
        result = f"""🌩️ **Generated {topology_type.title()} Storm Topology ({language.title()})**

**Requirements:** {requirements}

```{language}
{code}
```

**Key Components:**
• **Spout**: Data source that emits tuples into the topology
• **Bolt**: Processing component that consumes tuples and optionally emits new ones
• **Stream Grouping**: Defines how tuples are distributed between bolt instances
• **Topology**: Complete graph of spouts and bolts with stream groupings

**Next Steps:**
1. Customize the spout to read from your data source
2. Implement bolt processing logic for your use case
3. Configure appropriate parallelism levels
4. Test in local mode before deploying to cluster
5. Monitor performance and adjust as needed
"""
        
        return [types.TextContent(type="text", text=result)]
    
    async def run(self):
        """Run the MCP server"""
        async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
            # Create capabilities with tools support
            capabilities = types.ServerCapabilities(
                tools=types.ToolsCapability(listTools=True)
            )
            
            # Create initialization options
            init_options = InitializationOptions(
                server_name="storm-expert",
                server_version="1.0.0",
                capabilities=capabilities
            )
            
            await self.server.run(read_stream, write_stream, init_options)


async def main():
    """Main entry point"""
    print("🌩️ Starting Storm MCP Server...")
    if STORM_THRIFT_AVAILABLE:
        print("✅ Storm thrift bindings loaded successfully")
    else:
        print("⚠️  Storm thrift bindings not available")
    
    server = StormExpertServer()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main())