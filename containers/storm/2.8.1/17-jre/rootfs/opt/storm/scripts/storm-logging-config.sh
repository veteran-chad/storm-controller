#!/bin/bash

set -e
set -o pipefail

# Function to log with timestamp
log() {
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S UTC')] $*" >&2
}

# Get log format from environment variable
LOG_FORMAT="${LOG_FORMAT:-}"

# Storm log4j2 directory
STORM_LOG4J2_DIR="${STORM_HOME:-/apache-storm}/log4j2"
CONFIG_SOURCE_DIR="/opt/storm/configs"

# Function to copy log configuration files
copy_log_configs() {
    local format="$1"
    local source_dir="$CONFIG_SOURCE_DIR/$format"
    
    if [ ! -d "$source_dir" ]; then
        log "WARNING: Log format directory '$source_dir' not found"
        return 1
    fi
    
    # Copy cluster.xml if it exists
    if [ -f "$source_dir/cluster.xml" ]; then
        cp "$source_dir/cluster.xml" "$STORM_LOG4J2_DIR/cluster.xml"
        log "Copied cluster.xml for format: $format"
    else
        log "WARNING: $source_dir/cluster.xml not found"
    fi
    
    # Copy worker.xml if it exists
    if [ -f "$source_dir/worker.xml" ]; then
        cp "$source_dir/worker.xml" "$STORM_LOG4J2_DIR/worker.xml"
        log "Copied worker.xml for format: $format"
    else
        log "WARNING: $source_dir/worker.xml not found"
    fi
    
    return 0
}

# Main logic
log "Storm logging configuration starting..."

case "$LOG_FORMAT" in
    ""|"default"|"null")
        log "Using default Storm logging configuration (LOG_FORMAT=$LOG_FORMAT)"
        ;;
    "text")
        log "Configuring text logging format"
        if copy_log_configs "text"; then
            log "Text logging configuration applied successfully"
        else
            log "ERROR: Failed to apply text logging configuration"
            exit 1
        fi
        ;;
    *)
        log "Configuring custom logging format: $LOG_FORMAT"
        if copy_log_configs "$LOG_FORMAT"; then
            log "Custom logging configuration '$LOG_FORMAT' applied successfully"
        else
            log "WARNING: Failed to apply custom logging configuration '$LOG_FORMAT', using defaults"
        fi
        ;;
esac

log "Storm logging configuration completed"