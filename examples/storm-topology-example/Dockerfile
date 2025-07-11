FROM busybox:latest

# Copy the Storm starter JAR from Maven Central
ADD https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.4.0/storm-starter-2.4.0.jar /storm-starter.jar

# Set permissions
RUN chmod 644 /storm-starter.jar

# Default entrypoint
ENTRYPOINT ["sh", "-c", "echo 'Storm topology JAR container'; ls -la /storm-starter.jar; sleep infinity"]