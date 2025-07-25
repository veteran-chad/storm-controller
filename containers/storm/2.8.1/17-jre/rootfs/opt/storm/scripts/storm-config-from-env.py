#!/usr/bin/env python3

import os
import sys
import yaml
from collections import OrderedDict
from datetime import datetime, timezone

# Preserve YAML formatting
class OrderedDumper(yaml.SafeDumper):
    pass

def _dict_representer(dumper, data):
    return dumper.represent_mapping(
        yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
        data.items()
    )

OrderedDumper.add_representer(OrderedDict, _dict_representer)

# Track overridden keys
overridden_keys = []

def timestamp():
    """Get current UTC timestamp string."""
    return datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')

def env_to_storm_config(env_prefix="STORM_"):
    """
    Convert environment variables to Storm configuration.
    
    Environment variables should be prefixed with STORM_ and use double underscores
    for nested properties. Array values should be comma-separated.
    
    Examples:
    - STORM_NIMBUS_SEEDS=nimbus1,nimbus2 -> nimbus.seeds: ["nimbus1", "nimbus2"]
    - STORM_SUPERVISOR__SLOTS__PORTS=6700,6701,6702 -> supervisor.slots.ports: [6700, 6701, 6702]
    - STORM_UI__PORT=8080 -> ui.port: 8080
    - STORM_TOPOLOGY__MAX__SPOUT__PENDING=1000 -> topology.max.spout.pending: 1000
    """
    configs = []
    
    for key, value in sorted(os.environ.items()):
        if not key.startswith(env_prefix):
            continue
            
        # Remove prefix and convert to lowercase
        config_key = key[len(env_prefix):].lower()
        
        # Replace double underscores with dots
        config_key = config_key.replace('__', '.')
        
        # Parse the value
        parsed_value = parse_value(value)
        
        # Store as a flat key-value pair
        configs.append((config_key, parsed_value))
    
    return configs

def parse_value(value):
    """Parse environment variable value to appropriate type."""
    # Handle empty values
    if not value:
        return None
    
    # Handle boolean values
    if value.lower() in ('true', 'false'):
        return value.lower() == 'true'
    
    # Handle arrays (comma-separated)
    if ',' in value:
        items = [item.strip() for item in value.split(',')]
        # Try to parse each item
        parsed_items = []
        for item in items:
            parsed_items.append(parse_single_value(item))
        return parsed_items
    
    # Handle single values
    return parse_single_value(value)

def parse_single_value(value):
    """Parse a single value to appropriate type."""
    # Try integer
    try:
        return int(value)
    except ValueError:
        pass
    
    # Try float
    try:
        return float(value)
    except ValueError:
        pass
    
    # Return as string
    return value

def get_nested_value(config, key_path):
    """Get a value from nested dictionary using dot notation."""
    keys = key_path.split('.')
    current = config
    
    for k in keys:
        if isinstance(current, dict) and k in current:
            current = current[k]
        else:
            return None
    
    return current

def set_nested_value(config, key, value):
    """Set a value in nested dictionary using dot notation."""
    keys = key.split('.')
    current = config
    
    for k in keys[:-1]:
        if k not in current:
            current[k] = {}
        current = current[k]
    
    current[keys[-1]] = value


def main():
    global overridden_keys
    config_file = os.environ.get('STORM_CONF_DIR', '/conf') + '/storm.yaml'
    
    try:
        # Load existing config if it exists
        base_config = OrderedDict()
        if os.path.exists(config_file):
            try:
                with open(config_file, 'r') as f:
                    loaded = yaml.safe_load(f)
                    if loaded:
                        base_config = OrderedDict(loaded)
            except Exception as e:
                print(f"[{timestamp()}] ERROR: Failed to load existing config: {e}", file=sys.stderr)
                return 1
    
        # Get config from environment
        env_configs = env_to_storm_config()
        
        if env_configs:
            print(f"[{timestamp()}] Processing {len(env_configs)} configuration(s) from environment variables", file=sys.stderr)
            
            # Reset overridden keys tracker
            overridden_keys = []
            
            # Apply each configuration
            for config_key, parsed_value in env_configs:
                # Get existing value to check type compatibility
                existing_value = get_nested_value(base_config, config_key)
                
                # Type check - warn if types don't match
                if existing_value is not None:
                    # Special case: if existing is a list/dict, don't override with non-list/dict
                    if isinstance(existing_value, list) and not isinstance(parsed_value, list):
                        print(f"[{timestamp()}] WARNING: Skipping '{config_key}' - cannot override list with {type(parsed_value).__name__}", file=sys.stderr)
                        continue
                    elif isinstance(existing_value, dict) and not isinstance(parsed_value, dict):
                        print(f"[{timestamp()}] WARNING: Skipping '{config_key}' - cannot override dict with {type(parsed_value).__name__}", file=sys.stderr)
                        continue
                
                # Set the value and track override
                set_nested_value(base_config, config_key, parsed_value)
                overridden_keys.append(config_key)
            
            # Report what was overridden
            if overridden_keys:
                print(f"[{timestamp()}] Overridden configuration keys:", file=sys.stderr)
                for key in sorted(overridden_keys):
                    print(f"[{timestamp()}]   - {key}", file=sys.stderr)
        else:
            print(f"[{timestamp()}] No STORM_* environment variables found", file=sys.stderr)
        
        # Write the config file
        with open(config_file, 'w') as f:
            yaml.dump(dict(base_config), f, Dumper=OrderedDumper, default_flow_style=False, sort_keys=False)
        
        print(f"[{timestamp()}] Storm configuration written to {config_file}", file=sys.stderr)
        return 0
        
    except Exception as e:
        print(f"[{timestamp()}] ERROR: Unexpected error while processing configuration: {e}", file=sys.stderr)
        return 1

if __name__ == "__main__":
    sys.exit(main())