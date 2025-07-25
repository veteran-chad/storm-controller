# Apache Storm Docker Container

This Docker container provides a production-ready Apache Storm deployment with flexible configuration options through environment variables and build arguments.

## Features

- Apache Storm 2.8.1 running on Eclipse Temurin JRE 17
- Dynamic configuration through environment variables
- Flexible logging configuration (text/JSON formats)
- Secure user setup with proper permissions
- Automatic configuration validation with fail-fast behavior
- GPG signature verification for Storm distribution

## Build Arguments

The following arguments can be provided at build time using `--build-arg`:

| Argument | Description | Default |
|----------|-------------|---------|
| `TAG` | Base Eclipse Temurin image tag | `17-jre` |
| `VERSION` | Apache Storm version to install | `2.8.1` |
| `SERVICE` | Service name for identification | `storm` |
| `ENVIRONMENT` | Environment name (local/dev/staging/production) | `local` |

### Build Scripts

#### Production Build Script (`build.sh`)

A comprehensive build script that handles versioning and tagging:

```bash
# Basic build
./build.sh --version 2.8.1 --tag 17-jre

# Build with build ID
./build.sh --version 2.8.1 --tag 17-jre --build-id 20250725.1

# Build with custom registry
./build.sh --version 2.8.1 --tag 17-jre --registry myregistry.io

# Build without cache
./build.sh --version 2.8.1 --tag 17-jre --no-cache
```

The script will:
- Tag the image as `docker.io/storm:VERSION-TAG`
- Optionally add a build ID tag: `docker.io/storm:VERSION-TAG-BUILD_ID`
- Set ENVIRONMENT=production by default

#### Local Development Script (`build-local.sh`)

Quick build for local development:

```bash
# Build with defaults (2.8.1, 17-jre)
./build-local.sh

# Build with custom version
./build-local.sh 2.8.0

# Build with custom version and tag
./build-local.sh 2.8.0 11-jre
```

Tags the image as `storm-local` with ENVIRONMENT=local.

### Manual Build Commands

```bash
# Build with defaults
docker build -t storm:latest .

# Build with custom Storm version
docker build --build-arg VERSION=2.8.0 -t storm:2.8.0 .

# Build for production environment
docker build --build-arg ENVIRONMENT=production -t storm:production .
```

## Environment Variables

### Service Metadata

These variables are set from build arguments but can be overridden at runtime:

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `VERSION` | Storm version | `2.8.1` | `2.8.0` |
| `SERVICE` | Service identifier | `storm` | `storm-cluster` |
| `ENVIRONMENT` | Environment name | `local` | `production` |

### Storm Directories

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `STORM_CONF_DIR` | Configuration directory | `/conf` | `/opt/storm/conf` |
| `STORM_DATA_DIR` | Data directory | `/data` | `/var/storm/data` |
| `STORM_LOG_DIR` | Log directory | `/logs` | `/var/log/storm` |

### Logging Configuration

| Variable | Description | Default | Options |
|----------|-------------|---------|---------|
| `LOG_FORMAT` | Log output format | `text` | `text`, `json`, `default`, `null`, or custom |

#### Log Format Behavior

- **`text`**: Uses human-readable text format with timestamps
- **`json`**: Uses structured JSON format (includes service metadata)
- **`default`/`null`/`""`**: Uses Storm's default logging configuration
- **Custom**: Attempts to load configuration from `/opt/storm/configs/{format}/`

### Storm Configuration via Environment Variables

Any environment variable prefixed with `STORM_` will be converted to a storm.yaml configuration entry.

#### Naming Convention

- Environment variables must start with `STORM_`
- Use double underscores (`__`) to represent nested properties
- Variable names are converted to lowercase

#### Value Parsing

- **Booleans**: `true` or `false` (case-insensitive)
- **Arrays**: Comma-separated values (e.g., `value1,value2,value3`)
- **Numbers**: Automatically detected (integers and floats)
- **Strings**: Everything else

#### Configuration Examples

| Environment Variable | Resulting Configuration | Type |
|---------------------|------------------------|------|
| `STORM_UI__PORT=8080` | `ui.port: 8080` | Integer |
| `STORM_TOPOLOGY__DEBUG=true` | `topology.debug: true` | Boolean |
| `STORM_NIMBUS__SEEDS=nimbus1,nimbus2` | `nimbus.seeds: ["nimbus1", "nimbus2"]` | Array |
| `STORM_SUPERVISOR__SLOTS__PORTS=6700,6701,6702` | `supervisor.slots.ports: [6700, 6701, 6702]` | Array |
| `STORM_TOPOLOGY__MAX__SPOUT__PENDING=1000` | `topology.max.spout.pending: 1000` | Integer |
| `STORM_WORKER__CHILDOPTS="-Xmx768m"` | `worker.childopts: "-Xmx768m"` | String |

#### JMX Configuration Example

```bash
# Enable JMX for Nimbus
STORM_NIMBUS__CHILDOPTS="-Xmx1024m -Dcom.sun.management.jmxremote -Dcom.sun.management.jmxremote.port=9997 -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false"

# Enable JMX for Supervisor
STORM_SUPERVISOR__CHILDOPTS="-Xmx256m -Dcom.sun.management.jmxremote -Dcom.sun.management.jmxremote.port=9998 -Dcom.sun.management.jmxremote.authenticate=false -Dcom.sun.management.jmxremote.ssl=false"
```

## Container Behavior

### Startup Process

1. **User Permission Setup**: If running as root with `storm` command, switches to storm user
2. **Configuration Generation**: Creates default storm.yaml if not exists
3. **Environment Variable Processing**: Applies STORM_* environment variables to configuration
4. **Logging Configuration**: Configures log format based on LOG_FORMAT variable
5. **Custom Configuration Check**: If `/opt/storm/configs/storm/storm.yaml` exists, it overrides all previous configuration
6. **Command Execution**: Runs the specified command

### Error Handling

The container will fail to start if:
- Configuration file is malformed or unreadable
- Environment variable processing fails
- Logging configuration fails
- Required directories cannot be created

### Default Configuration

If no storm.yaml exists, the container creates one with:

```yaml
storm.zookeeper.servers: [zookeeper]
nimbus.seeds: [nimbus]
storm.log.dir: "/logs"
storm.local.dir: "/data"
```

## Usage Examples

### Running Nimbus

```bash
docker run -d \
  --name storm-nimbus \
  -e STORM_NIMBUS__SEEDS=nimbus1,nimbus2 \
  -e STORM_UI__PORT=8080 \
  -e LOG_FORMAT=json \
  storm:latest storm nimbus
```

### Running Supervisor

```bash
docker run -d \
  --name storm-supervisor \
  -e STORM_NIMBUS__SEEDS=nimbus1,nimbus2 \
  -e STORM_SUPERVISOR__SLOTS__PORTS=6700,6701,6702,6703 \
  -e LOG_FORMAT=json \
  storm:latest storm supervisor
```

### Running UI

```bash
docker run -d \
  --name storm-ui \
  -p 8080:8080 \
  -e STORM_NIMBUS__SEEDS=nimbus1,nimbus2 \
  -e STORM_UI__PORT=8080 \
  storm:latest storm ui
```

### Custom Configuration File

```bash
# Method 1: Direct mount to /conf/storm.yaml (bypasses all configuration scripts)
docker run -d \
  -v /path/to/storm.yaml:/conf/storm.yaml \
  -e LOG_FORMAT=text \
  storm:latest storm nimbus

# Method 2: Mount to override directory (applies after environment variables)
docker run -d \
  -v /path/to/storm.yaml:/opt/storm/configs/storm/storm.yaml \
  -e LOG_FORMAT=json \
  storm:latest storm nimbus
```

## Scripts and Tools

The container includes several utility scripts in `/opt/storm/scripts/`:

- **`docker-entrypoint.sh`**: Main entrypoint script
- **`storm-config-from-env.py`**: Processes environment variables into storm.yaml
- **`storm-logging-config.sh`**: Configures logging format
- **`storm-custom-config.sh`**: Applies custom storm.yaml if present in `/opt/storm/configs/storm/`

## Logging Output

All startup and configuration logs use a consistent format:

```
[YYYY-MM-DD HH:MM:SS UTC] Log message
```

Configuration changes are logged with details:

```
[2025-07-25 12:00:00 UTC] Processing 3 configuration(s) from environment variables
[2025-07-25 12:00:00 UTC] Overridden configuration keys:
[2025-07-25 12:00:00 UTC]   - nimbus.seeds
[2025-07-25 12:00:00 UTC]   - ui.port
[2025-07-25 12:00:00 UTC]   - topology.debug
```

## Type Safety

The configuration system preserves existing types and warns when attempting incompatible overrides:

```
[2025-07-25 12:00:00 UTC] WARNING: Skipping 'topology.kryo.register' - cannot override list with string
```

## Security Considerations

- Container runs as non-root user (storm:storm with UID/GID 1000)
- GPG signature verification for Storm distribution
- No default JMX ports exposed (must be explicitly configured)
- Sensitive configuration should be provided via secrets/configs, not environment variables

## Troubleshooting

### Container Fails to Start

Check logs for configuration errors:
```bash
docker logs <container-name>
```

### Configuration Not Applied

Ensure environment variables:
- Start with `STORM_` prefix
- Use double underscores for nested properties
- Have valid values for the expected type

### Logging Issues

Verify LOG_FORMAT value and check if custom format directory exists:
```bash
docker exec <container-name> ls -la /opt/storm/configs/
```

## License

This Docker image includes Apache Storm, which is licensed under the Apache License 2.0.