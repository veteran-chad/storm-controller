#!/bin/bash

set -e  # Exit on any error
set -o pipefail  # Exit on pipe failures

# Allow the container to be started with `--user`
if [ "$1" = 'storm' -a "$(id -u)" = '0' ]; then
    chown -R storm:storm "$STORM_CONF_DIR" "$STORM_DATA_DIR" "$STORM_LOG_DIR"
    exec gosu storm "$0" "$@"
fi

# Generate the config only if it doesn't exist
CONFIG="$STORM_CONF_DIR/storm.yaml"
if [ ! -f "$CONFIG" ]; then
    cat << EOF > "$CONFIG"
storm.zookeeper.servers: [zookeeper]
nimbus.seeds: [nimbus]
storm.log.dir: "$STORM_LOG_DIR"
storm.local.dir: "$STORM_DATA_DIR"
EOF
fi

# Apply configuration from environment variables
echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] Applying Storm configuration from environment variables..."
if ! storm-config-from-env.py; then
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] ERROR: Failed to apply Storm configuration from environment variables" >&2
    exit 1
fi

# Apply logging configuration
echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] Applying Storm logging configuration..."
if ! storm-logging-config.sh; then
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] ERROR: Failed to apply Storm logging configuration" >&2
    exit 1
fi

# Check for custom configuration override
echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] Checking for custom Storm configuration..."
if ! storm-custom-config.sh; then
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] ERROR: Failed to apply custom Storm configuration" >&2
    exit 1
fi

exec "$@"