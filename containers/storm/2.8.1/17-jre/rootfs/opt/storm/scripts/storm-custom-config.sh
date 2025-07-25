#!/bin/bash

set -e
set -o pipefail

# Function to log with timestamp
log() {
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] $*" >&2
}

# Custom config directory
CUSTOM_CONFIG_DIR="/opt/storm/configs/storm"
CUSTOM_CONFIG_FILE="$CUSTOM_CONFIG_DIR/storm.yaml"
STORM_CONFIG_FILE="${STORM_CONF_DIR:-/conf}/storm.yaml"

# Check if custom storm.yaml exists
if [ -f "$CUSTOM_CONFIG_FILE" ]; then
    log "Custom storm.yaml found at $CUSTOM_CONFIG_FILE"
    
    # Backup existing config if it exists
    if [ -f "$STORM_CONFIG_FILE" ]; then
        log "Backing up existing configuration to ${STORM_CONFIG_FILE}.bak"
        cp "$STORM_CONFIG_FILE" "${STORM_CONFIG_FILE}.bak"
    fi
    
    # Copy custom config
    log "Applying custom storm.yaml configuration"
    cp "$CUSTOM_CONFIG_FILE" "$STORM_CONFIG_FILE"
    
    # Set proper ownership
    if [ "$(id -u)" = '0' ]; then
        chown storm:storm "$STORM_CONFIG_FILE"
    fi
    
    log "Custom configuration applied successfully"
else
    log "No custom storm.yaml found at $CUSTOM_CONFIG_FILE, keeping generated configuration"
fi