# Base image for Storm topology JAR deployments
# This Dockerfile provides a minimal container for packaging Storm topology JARs
# to be extracted and deployed by the Storm Kubernetes controller

# Use a minimal base image with Java runtime
FROM eclipse-temurin:17-jre-alpine AS base

# Add labels for better container management
LABEL org.opencontainers.image.title="Storm Topology JAR Container"
LABEL org.opencontainers.image.description="Base container for Storm topology JAR deployment"
LABEL org.opencontainers.image.vendor="Apache Storm"
LABEL org.opencontainers.image.source="https://github.com/veteran-chad/storm-controller"

# Install required packages
RUN apk add --no-cache \
    bash \
    curl \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1000 storm && \
    adduser -D -u 1000 -G storm storm

# Create directory structure
RUN mkdir -p /storm/jars && \
    chown -R storm:storm /storm

# Switch to non-root user
USER storm
WORKDIR /storm

# Default JAR location - this should be overridden when building
# specific topology images
ENV TOPOLOGY_JAR_PATH=/storm/jars/topology.jar

# Add entrypoint script for flexibility
COPY --chown=storm:storm entrypoint.sh /storm/
RUN chmod +x /storm/entrypoint.sh

# The actual topology JAR should be added in child Dockerfiles
# Example:
# COPY --chown=storm:storm my-topology.jar /storm/jars/topology.jar

# Set a default command that can be overridden
ENTRYPOINT ["/storm/entrypoint.sh"]
CMD ["sleep", "infinity"]

# Multi-stage build example for creating topology-specific images
# FROM base AS my-topology
# COPY --chown=storm:storm target/my-topology-1.0.0.jar /storm/jars/topology.jar
# ENV TOPOLOGY_NAME="my-topology"
# ENV TOPOLOGY_MAIN_CLASS="com.example.MyTopology"