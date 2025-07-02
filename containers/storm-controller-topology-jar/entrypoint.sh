#!/bin/bash
# Entrypoint script for Storm topology JAR container
# This script provides flexibility for JAR extraction and validation

set -e

# Function to log messages
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

# Function to calculate checksum
calculate_checksum() {
    local file=$1
    local type=${2:-sha256}
    
    case $type in
        sha256)
            sha256sum "$file" | awk '{print $1}'
            ;;
        sha512)
            sha512sum "$file" | awk '{print $1}'
            ;;
        md5)
            md5sum "$file" | awk '{print $1}'
            ;;
        *)
            log "ERROR: Unknown checksum type: $type"
            return 1
            ;;
    esac
}

# Function to validate JAR file
validate_jar() {
    local jar_file=$1
    
    # Check if file exists
    if [ ! -f "$jar_file" ]; then
        log "ERROR: JAR file not found: $jar_file"
        return 1
    fi
    
    # Check if it's a valid JAR
    if ! jar tf "$jar_file" > /dev/null 2>&1; then
        log "ERROR: Invalid JAR file: $jar_file"
        return 1
    fi
    
    log "JAR validation successful: $jar_file"
    return 0
}

# Main execution
log "Storm topology JAR container starting..."

# Check if running in extraction mode
if [ "$1" = "extract" ]; then
    log "Running in extraction mode"
    
    # Validate environment variables
    if [ -z "$EXTRACTION_PATH" ]; then
        log "ERROR: EXTRACTION_PATH not set"
        exit 1
    fi
    
    # Ensure extraction directory exists
    mkdir -p "$EXTRACTION_PATH"
    
    # Copy JAR to extraction path
    if [ -f "$TOPOLOGY_JAR_PATH" ]; then
        log "Copying JAR from $TOPOLOGY_JAR_PATH to $EXTRACTION_PATH/"
        cp "$TOPOLOGY_JAR_PATH" "$EXTRACTION_PATH/"
        
        # Calculate and log checksum
        jar_name=$(basename "$TOPOLOGY_JAR_PATH")
        checksum=$(calculate_checksum "$EXTRACTION_PATH/$jar_name")
        log "JAR checksum (SHA256): $checksum"
        
        # Write checksum file if requested
        if [ -n "$WRITE_CHECKSUM" ]; then
            echo "$checksum" > "$EXTRACTION_PATH/$jar_name.sha256"
            log "Checksum written to $EXTRACTION_PATH/$jar_name.sha256"
        fi
        
        # Validate the copied JAR
        validate_jar "$EXTRACTION_PATH/$jar_name"
        
        log "JAR extraction completed successfully"
    else
        log "ERROR: Topology JAR not found at $TOPOLOGY_JAR_PATH"
        exit 1
    fi
    
elif [ "$1" = "validate" ]; then
    log "Running in validation mode"
    
    # Validate the JAR at the expected location
    validate_jar "$TOPOLOGY_JAR_PATH"
    
    # If checksum provided, validate it
    if [ -n "$EXPECTED_CHECKSUM" ] && [ -n "$CHECKSUM_TYPE" ]; then
        actual_checksum=$(calculate_checksum "$TOPOLOGY_JAR_PATH" "$CHECKSUM_TYPE")
        if [ "$actual_checksum" = "$EXPECTED_CHECKSUM" ]; then
            log "Checksum validation successful"
        else
            log "ERROR: Checksum mismatch"
            log "Expected: $EXPECTED_CHECKSUM"
            log "Actual: $actual_checksum"
            exit 1
        fi
    fi
    
elif [ "$1" = "info" ]; then
    log "Running in info mode"
    
    # Display information about the JAR
    if [ -f "$TOPOLOGY_JAR_PATH" ]; then
        log "JAR Information:"
        log "  Path: $TOPOLOGY_JAR_PATH"
        log "  Size: $(du -h "$TOPOLOGY_JAR_PATH" | awk '{print $1}')"
        log "  SHA256: $(calculate_checksum "$TOPOLOGY_JAR_PATH" sha256)"
        
        # List main classes if possible
        log "  Main classes:"
        jar tf "$TOPOLOGY_JAR_PATH" | grep -E "\.class$" | grep -v "$" | head -20 | sed 's/\.class$//' | sed 's/\//./g' | sed 's/^/    /'
    else
        log "ERROR: JAR file not found at $TOPOLOGY_JAR_PATH"
        exit 1
    fi
    
else
    # Default behavior - run the provided command
    exec "$@"
fi