# Example Dockerfile for packaging a Storm topology
FROM busybox:latest AS builder

# In a real scenario, you would build your topology here
# For example, with Maven:
# FROM maven:3.8-openjdk-11 AS builder
# COPY pom.xml .
# COPY src ./src
# RUN mvn clean package

# For this example, we'll just create a placeholder
RUN mkdir -p /build && \
    echo "This would be your topology JAR" > /build/placeholder.txt

# Final stage - minimal image with just the JAR
FROM busybox:latest
COPY --from=builder /build /app

# In a real topology image:
# COPY --from=builder /target/my-topology-1.0.0.jar /app/my-topology.jar

# The controller will extract the JAR from this path
LABEL storm.topology.jar.path="/app/my-topology.jar"