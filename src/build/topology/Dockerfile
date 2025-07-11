FROM docker.io/veteranchad/storm-controller-topology-jar:latest

# Copy the storm-starter JAR into the container
COPY --chown=storm:storm storm-starter-2.8.1.jar /storm/jars/topology.jar

# Set metadata
ENV TOPOLOGY_NAME="storm-starter"
ENV TOPOLOGY_VERSION="2.8.1"