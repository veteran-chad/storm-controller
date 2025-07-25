#!/bin/bash

set -e  # Exit on any error
set -o pipefail  # Exit on pipe failures

# Allow the container to be started with `--user`
if [ "$1" = 'storm' -a "$(id -u)" = '0' ]; then
    chown -R storm:storm "$STORM_CONF_DIR" "$STORM_DATA_DIR" "$STORM_LOG_DIR"
    exec gosu storm "$0" "$@"
fi

# Don't create a default config - let the Python script generate it completely
# This avoids type conflicts when environment variables override arrays
CONFIG="$STORM_CONF_DIR/storm.yaml"

# Apply configuration from environment variables
echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] Applying Storm configuration from environment variables..."
if ! storm-config-from-env.py; then
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] ERROR: Failed to apply Storm configuration from environment variables" >&2
    exit 1
fi

# Create symlink to Storm's expected config location
if [ -f "$CONFIG" ]; then
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] Creating symlink from $CONFIG to /apache-storm/conf/storm.yaml"
    ln -sf "$CONFIG" /apache-storm/conf/storm.yaml
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