# Production Deployment Guide for Storm Kubernetes

This guide covers deploying Apache Storm on Kubernetes for production use with all security, monitoring, and high-availability features enabled.

## Prerequisites

1. **Kubernetes Cluster**
   - Version 1.24+ recommended
   - At least 3 worker nodes for high availability
   - Nodes with at least 8 CPU cores and 32GB RAM each

2. **Storage**
   - Fast SSD storage class available (e.g., `fast-ssd`)
   - Support for ReadWriteOnce persistent volumes

3. **Networking**
   - NGINX Ingress Controller installed
   - cert-manager installed for TLS certificates
   - DNS configured for your Storm UI hostname

4. **Monitoring**
   - Prometheus Operator installed
   - Grafana installed for dashboards
   - metrics-server installed for HPA

5. **External Dependencies**
   - Zookeeper cluster deployed (3+ nodes)

## Pre-deployment Setup

### 1. Create Namespace

```bash
kubectl create namespace storm-production
kubectl label namespace storm-production environment=production
```

### 2. Create Image Pull Secret (if using private registry)

```bash
kubectl create secret docker-registry storm-pull-secret \
  --docker-server=your-registry.com \
  --docker-username=your-username \
  --docker-password=your-password \
  --namespace storm-production
```

### 3. Deploy Zookeeper (if not already deployed)

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install zookeeper bitnami/zookeeper \
  --namespace zookeeper \
  --create-namespace \
  --set replicaCount=3 \
  --set persistence.enabled=true \
  --set persistence.size=10Gi
```

### 4. Create TLS Certificate

Using cert-manager with Let's Encrypt:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

## Deployment

### 1. Review and Customize Values

Copy and customize the production values:

```bash
cp values-production.yaml my-production-values.yaml
```

Key configurations to review:
- `global.imageRegistry`: Set if using private registry
- `externalZookeeper.servers`: Update with your Zookeeper endpoints
- `ui.ingress.hostname`: Set your production hostname
- Resource limits and requests for all components
- Storage class names
- Monitoring labels to match your Prometheus setup

### 2. Deploy Storm

```bash
helm install storm-prod . \
  -f my-production-values.yaml \
  --namespace storm-production \
  --create-namespace
```

### 3. Verify Deployment

Check all pods are running:

```bash
kubectl get pods -n storm-production
```

Expected output:
```
NAME                                          READY   STATUS    RESTARTS   AGE
storm-prod-nimbus-0                          1/1     Running   0          5m
storm-prod-nimbus-1                          1/1     Running   0          5m
storm-prod-nimbus-2                          1/1     Running   0          5m
storm-prod-supervisor-xxxxx                  1/1     Running   0          5m
storm-prod-supervisor-xxxxx                  1/1     Running   0          5m
storm-prod-supervisor-xxxxx                  1/1     Running   0          5m
storm-prod-supervisor-xxxxx                  1/1     Running   0          5m
storm-prod-supervisor-xxxxx                  1/1     Running   0          5m
storm-prod-ui-xxxxx                          1/1     Running   0          5m
storm-prod-ui-xxxxx                          1/1     Running   0          5m
storm-prod-metrics-exporter-xxxxx            1/1     Running   0          5m
```

### 4. Verify Services

```bash
kubectl get svc -n storm-production
```

### 5. Check Ingress

```bash
kubectl get ingress -n storm-production
```

The ingress should show your hostname and the load balancer IP.

## Post-deployment Configuration

### 1. Configure Monitoring

Import the Grafana dashboard:

```bash
kubectl apply -f storm-grafana-dashboard.yaml
```

Verify metrics are being collected:

```bash
curl -s http://storm-prod-metrics:9102/metrics | grep storm_
```

### 2. Configure Autoscaling

The HPA is automatically created if you have metrics-server installed. Verify:

```bash
kubectl get hpa -n storm-production
```

### 3. Test High Availability

Test nimbus failover:

```bash
# Delete one nimbus pod
kubectl delete pod storm-prod-nimbus-0 -n storm-production

# Verify it's recreated and cluster remains operational
kubectl get pods -n storm-production
```

### 4. Security Verification

Check network policies:

```bash
kubectl get networkpolicies -n storm-production
```

Verify pod security contexts:

```bash
kubectl get pod storm-prod-nimbus-0 -n storm-production -o yaml | grep -A10 securityContext
```

## Operational Procedures

### 1. Scaling Supervisors

Manual scaling:

```bash
kubectl scale deployment storm-prod-supervisor --replicas=10 -n storm-production
```

Or update the values file and upgrade:

```yaml
supervisor:
  replicaCount: 10
```

```bash
helm upgrade storm-prod . -f my-production-values.yaml -n storm-production
```

### 2. Rolling Updates

Storm supports rolling updates with zero downtime:

```bash
helm upgrade storm-prod . \
  -f my-production-values.yaml \
  --namespace storm-production
```

The PodDisruptionBudgets ensure availability during updates.

### 3. Backup and Recovery

Backup Nimbus data:

```bash
# Create backup job
kubectl create job nimbus-backup --from=cronjob/nimbus-backup -n storm-production
```

### 4. Monitoring and Alerts

Key metrics to monitor:
- `storm_topologies_total`: Number of running topologies
- `storm_executors_total`: Total executors
- `storm_slots_used`: Used worker slots
- `storm_nimbus_uptime`: Nimbus uptime

Key alerts configured:
- Storm cluster down
- High slot usage (>90%)
- Nimbus not available
- Supervisor failures

## Troubleshooting

### 1. Check Logs

```bash
# Nimbus logs
kubectl logs -f storm-prod-nimbus-0 -n storm-production

# Supervisor logs
kubectl logs -f deployment/storm-prod-supervisor -n storm-production

# UI logs
kubectl logs -f deployment/storm-prod-ui -n storm-production
```

### 2. Access Storm UI

Port-forward for debugging:

```bash
kubectl port-forward svc/storm-prod-ui 8080:8080 -n storm-production
```

Or access via the ingress URL: https://storm.production.example.com

### 3. Common Issues

**Issue**: Supervisors can't connect to Nimbus
- Check network policies
- Verify Nimbus service is accessible
- Check Zookeeper connectivity

**Issue**: High memory usage
- Review autoMemory settings
- Check topology resource allocations
- Monitor GC logs

**Issue**: Topology submission fails
- Check nimbus logs
- Verify sufficient supervisor slots
- Check disk space on nimbus pods

## Maintenance

### 1. Certificate Renewal

If using cert-manager, certificates are auto-renewed. To manually renew:

```bash
kubectl delete certificate storm-prod-tls -n storm-production
```

### 2. Cleaning Old Logs

Supervisor logs can accumulate. Clean periodically:

```bash
kubectl exec -it deployment/storm-prod-supervisor -n storm-production -- \
  find /logs -name "*.log" -mtime +7 -delete
```

### 3. Zookeeper Maintenance

Ensure Zookeeper is regularly maintained:
- Monitor disk usage
- Clean old snapshots
- Regular backups

## Performance Tuning

### 1. JVM Tuning

For large deployments, adjust JVM settings:

```yaml
nimbus:
  jvmOptions: "-Xms4g -Xmx4g -XX:+UseG1GC -XX:MaxGCPauseMillis=200 -XX:+ParallelRefProcEnabled"

supervisor:
  jvmOptions: "-Xms2g -Xmx2g -XX:+UseG1GC -XX:MaxGCPauseMillis=100"
```

### 2. Network Tuning

For high-throughput topologies:

```yaml
stormConfig:
  storm.messaging.netty.buffer_size: 10485760  # 10MB
  storm.messaging.netty.server_worker_threads: 2
  storm.messaging.netty.client_worker_threads: 2
```

### 3. Topology Optimization

Best practices for topology configuration:
- Set appropriate `topology.max.spout.pending`
- Configure proper parallelism hints
- Use topology.worker.max.heap.size.mb

## Security Hardening

### 1. Enable Authentication

For Kerberos/SASL authentication, add to stormConfig:

```yaml
stormConfig:
  storm.thrift.transport: "org.apache.storm.security.auth.kerberos.KerberosSaslTransportPlugin"
  java.security.auth.login.config: "/etc/storm/storm_jaas.conf"
```

### 2. Restrict Network Access

Tighten network policies:

```yaml
networkPolicy:
  allowExternalUI: false  # Only allow through ingress
  allowExternalZookeeper: false
  allowedNamespaces:
    - storm-production
```

### 3. Regular Security Updates

- Keep Storm image updated
- Regularly update Kubernetes cluster
- Monitor security advisories

## Disaster Recovery

### 1. Multi-Region Setup

For DR, deploy Storm in multiple regions with:
- Shared Zookeeper (with observers)
- Cross-region network connectivity
- Regular nimbus data sync

### 2. Backup Strategy

Regular backups should include:
- Nimbus persistent data
- Storm configuration
- Deployed topology JARs
- Zookeeper data

### 3. Recovery Procedures

In case of complete cluster failure:
1. Deploy fresh Storm cluster
2. Restore Zookeeper data
3. Restore Nimbus data
4. Restart topologies

## Conclusion

This production deployment provides:
- High availability with multiple Nimbus nodes
- Automatic scaling of supervisors
- Comprehensive monitoring and alerting
- Security hardening with RBAC and network policies
- TLS encryption for all external traffic

Regular maintenance and monitoring ensure optimal performance and reliability for your Storm workloads.