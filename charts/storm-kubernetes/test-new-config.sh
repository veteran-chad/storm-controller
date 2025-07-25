#!/bin/bash

# Test script for validating the new Storm configuration approach
# This script tests both environment variable configuration and optional storm.yaml

set -e

echo "=== Storm Kubernetes Helm Chart Configuration Test ==="
echo

# Test 1: Basic configuration with environment variables only
echo "Test 1: Basic configuration with environment variables only"
echo "-------------------------------------------------------------"

cat > /tmp/test-env-only.yaml << 'EOF'
# Image tags updated to new version
nimbus:
  image:
    tag: 2.8.1-17-jre
  extraConfig:
    nimbus.childopts: "-Xmx2048m"
    nimbus.task.timeout.secs: 45

supervisor:
  image:
    tag: 2.8.1-17-jre
  slotsPerSupervisor: 2
  extraConfig:
    supervisor.childopts: "-Xmx512m"

ui:
  image:
    tag: 2.8.1-17-jre
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"

cluster:
  extraConfig:
    storm.log.level: "DEBUG"
    topology.workers: 3
EOF

echo "Running helm template with env-only configuration..."
helm template test-env . -f /tmp/test-env-only.yaml --namespace storm-system > /tmp/test-env-only-output.yaml

# Check for configmap-env
echo -n "✓ Checking for environment ConfigMap... "
if grep -q "name: test-env-storm-kubernetes-env" /tmp/test-env-only-output.yaml; then
    echo "PASS"
else
    echo "FAIL - Environment ConfigMap not found"
    exit 1
fi

# Check that storm.yaml ConfigMap is NOT created
echo -n "✓ Checking that storm.yaml ConfigMap is NOT created... "
# Look for the storm.yaml content, not just the name
if ! grep -q "storm.yaml: |" /tmp/test-env-only-output.yaml; then
    echo "PASS"
else
    echo "FAIL - storm.yaml ConfigMap should not exist"
    exit 1
fi

# Check environment variables in ConfigMap
echo -n "✓ Checking environment variables in ConfigMap... "
if grep -q "STORM_NIMBUS__CHILDOPTS: \"-Xmx2048m\"" /tmp/test-env-only-output.yaml && \
   grep -q "STORM_NIMBUS__TASK__TIMEOUT__SECS: \"45\"" /tmp/test-env-only-output.yaml && \
   grep -q "STORM_STORM__LOG__LEVEL: \"DEBUG\"" /tmp/test-env-only-output.yaml && \
   grep -q "STORM_TOPOLOGY__WORKERS: \"3\"" /tmp/test-env-only-output.yaml; then
    echo "PASS"
else
    echo "FAIL - Environment variables not correctly set"
    exit 1
fi

# Check supervisor slots configuration
echo -n "✓ Checking supervisor slots configuration... "
if grep -q "STORM_SUPERVISOR__SLOTS__PORTS: \"6700,6701\"" /tmp/test-env-only-output.yaml; then
    echo "PASS"
else
    echo "FAIL - Supervisor slots not configured correctly"
    exit 1
fi

echo

# Test 2: Configuration with custom storm.yaml
echo "Test 2: Configuration with custom storm.yaml"
echo "--------------------------------------------"

cat > /tmp/test-with-storm-yaml.yaml << 'EOF'
# Image tags updated to new version
nimbus:
  image:
    tag: 2.8.1-17-jre

supervisor:
  image:
    tag: 2.8.1-17-jre

ui:
  image:
    tag: 2.8.1-17-jre

cluster:
  # This should create the storm.yaml ConfigMap
  stormYaml: |
    storm.zookeeper.servers:
      - "custom-zk-1"
      - "custom-zk-2"
    nimbus.seeds:
      - "custom-nimbus-1"
      - "custom-nimbus-2"
    storm.log.dir: "/custom/logs"
    storm.local.dir: "/custom/data"
    ui.port: 8090
  # These should still go to env ConfigMap
  extraConfig:
    topology.max.spout.pending: 1000
    topology.message.timeout.secs: 60
EOF

echo "Running helm template with storm.yaml configuration..."
helm template test-yaml . -f /tmp/test-with-storm-yaml.yaml --namespace storm-system > /tmp/test-storm-yaml-output.yaml

# Check for both ConfigMaps
echo -n "✓ Checking for environment ConfigMap... "
if grep -q "name: test-yaml-storm-kubernetes-env" /tmp/test-storm-yaml-output.yaml; then
    echo "PASS"
else
    echo "FAIL - Environment ConfigMap not found"
    exit 1
fi

echo -n "✓ Checking for storm.yaml ConfigMap... "
# Check for the storm.yaml content which indicates the ConfigMap exists
if grep -q "storm.yaml: |" /tmp/test-storm-yaml-output.yaml; then
    echo "PASS"
else
    echo "FAIL - storm.yaml ConfigMap not found"
    exit 1
fi

# Check storm.yaml content
echo -n "✓ Checking storm.yaml content... "
if grep -A10 "storm.yaml:" /tmp/test-storm-yaml-output.yaml | grep -q "custom-zk-1" && \
   grep -A10 "storm.yaml:" /tmp/test-storm-yaml-output.yaml | grep -q "ui.port: 8090"; then
    echo "PASS"
else
    echo "FAIL - storm.yaml content incorrect"
    exit 1
fi

# Check that config volume is mounted
echo -n "✓ Checking config volume mount in nimbus... "
if sed -n '/name: nimbus$/,/volumes:/p' /tmp/test-storm-yaml-output.yaml | grep -q "mountPath: /conf"; then
    echo "PASS"
else
    echo "FAIL - Config volume not mounted in nimbus"
    exit 1
fi

echo

# Test 3: Memory auto-calculation
echo "Test 3: Memory auto-calculation"
echo "--------------------------------"

cat > /tmp/test-memory-auto.yaml << 'EOF'
supervisor:
  image:
    tag: 2.8.1-17-jre
  slotsPerSupervisor: 4
  memoryConfig:
    mode: "auto"
    memoryPerWorker: "2Gi"
    cpuPerWorker: "0.5"
EOF

echo "Running helm template with auto memory configuration..."
helm template test-mem . -f /tmp/test-memory-auto.yaml --namespace storm-system > /tmp/test-memory-output.yaml

# Check calculated memory values
echo -n "✓ Checking auto-calculated memory settings... "
# Should be 4 workers * 2Gi = 8192MB capacity
# Worker heap = 75% of 2Gi = 75% of 2048MB = 1536MB
if grep -q "STORM_SUPERVISOR__MEMORY__CAPACITY__MB: \"8192\"" /tmp/test-memory-output.yaml && \
   grep -q "STORM_SUPERVISOR__CPU__CAPACITY: \"200\"" /tmp/test-memory-output.yaml && \
   grep -q "STORM_WORKER__HEAP__MEMORY__MB: \"1536\"" /tmp/test-memory-output.yaml; then
    echo "PASS"
else
    echo "FAIL - Memory calculation incorrect"
    exit 1
fi

# Check container resources
echo -n "✓ Checking container resources... "
# Should be 4 * 2Gi * 1.25 = 10Gi = 10240Mi
if grep -A5 "resources:" /tmp/test-memory-output.yaml | grep -q "memory: 10240Mi" && \
   grep -A5 "resources:" /tmp/test-memory-output.yaml | grep -q "cpu: 2"; then
    echo "PASS"
else
    echo "FAIL - Container resources incorrect"
    exit 1
fi

echo

# Test 4: LOG_FORMAT propagation
echo "Test 4: LOG_FORMAT environment variable"
echo "---------------------------------------"

cat > /tmp/test-log-format.yaml << 'EOF'
nimbus:
  image:
    tag: 2.8.1-17-jre
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"

supervisor:
  image:
    tag: 2.8.1-17-jre
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"

ui:
  image:
    tag: 2.8.1-17-jre
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"
EOF

echo "Running helm template with LOG_FORMAT configuration..."
helm template test-log . -f /tmp/test-log-format.yaml --namespace storm-system > /tmp/test-log-output.yaml

# Check LOG_FORMAT in each component
echo -n "✓ Checking LOG_FORMAT in nimbus... "
if grep -A30 "name: nimbus" /tmp/test-log-output.yaml | grep -A2 "name: LOG_FORMAT" | grep -q "value: json"; then
    echo "PASS"
else
    echo "FAIL - LOG_FORMAT not set in nimbus"
    exit 1
fi

echo -n "✓ Checking LOG_FORMAT in supervisor... "
if grep -A30 "name: supervisor" /tmp/test-log-output.yaml | grep -A2 "name: LOG_FORMAT" | grep -q "value: json"; then
    echo "PASS"
else
    echo "FAIL - LOG_FORMAT not set in supervisor"
    exit 1
fi

echo -n "✓ Checking LOG_FORMAT in ui... "
if grep -A30 "name: ui" /tmp/test-log-output.yaml | grep -A2 "name: LOG_FORMAT" | grep -q "value: json"; then
    echo "PASS"
else
    echo "FAIL - LOG_FORMAT not set in ui"
    exit 1
fi

echo

# Test 5: Backward compatibility
echo "Test 5: Backward compatibility check"
echo "------------------------------------"

# Test that old values still work (without the new storm.yaml option)
echo -n "✓ Checking backward compatibility... "
helm template test-compat . -f storm-local-values.yaml --namespace storm-system > /tmp/test-compat-output.yaml 2>&1
if [ $? -eq 0 ]; then
    echo "PASS"
else
    echo "FAIL - Backward compatibility broken"
    exit 1
fi

echo
echo "=== All tests passed! ==="
echo
echo "Summary:"
echo "- Environment variables are correctly set in ConfigMap"
echo "- storm.yaml ConfigMap is only created when cluster.stormYaml is provided"
echo "- Memory auto-calculation works correctly"
echo "- LOG_FORMAT can be set per component"
echo "- Backward compatibility is maintained"
echo
echo "The new configuration approach is working correctly!"