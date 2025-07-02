# Example Dockerfile for Storm Topology JAR Container
# This creates a minimal container image containing only the topology JAR file

FROM scratch

# Copy the topology JAR file
COPY target/wordcount-topology-1.0.jar /app/topology.jar

# Optional: Copy dependency libraries
COPY target/lib/ /app/lib/

# Optional: Copy configuration files
COPY src/main/resources/topology.yaml /app/config/

# Metadata labels
LABEL org.apache.storm.topology.name="wordcount"
LABEL org.apache.storm.topology.version="1.0"
LABEL org.apache.storm.topology.main-class="com.example.WordCountTopology"