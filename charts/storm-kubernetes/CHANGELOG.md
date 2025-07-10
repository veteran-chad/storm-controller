# Changelog

## [0.2.0] - Production Readiness Release

### Added

#### Security Features
- **RBAC and ServiceAccounts**: Complete RBAC implementation with proper role bindings
- **Pod Security Contexts**: Non-root user (UID 1000), dropped capabilities, seccomp profiles
- **Container Security Contexts**: Read-only root filesystem support, privilege escalation prevention
- **Network Policies**: Granular network isolation with configurable ingress/egress rules
- **PodSecurityPolicy**: Optional PSP support for clusters requiring it
- **Authentication**: Full Kerberos/SASL support with JAAS configuration

#### High Availability
- **PodDisruptionBudgets**: For all components (Nimbus, Supervisor, UI)
- **Multi-Nimbus Support**: Configured for 3 Nimbus nodes in production
- **Anti-affinity Rules**: Spread pods across failure domains

#### Monitoring and Observability
- **Metrics Exporter**: Custom Python-based exporter for Storm metrics
- **ServiceMonitor**: Prometheus Operator integration
- **PrometheusRule**: Pre-configured alerts for critical conditions
- **Grafana Dashboard**: Comprehensive Storm cluster monitoring dashboard

#### Scaling
- **HorizontalPodAutoscaler**: CPU/Memory based autoscaling for supervisors
- **Custom Metrics Support**: Scale based on Storm-specific metrics
- **Configurable Behavior**: Fine-tuned scale up/down policies

#### Networking
- **Ingress Support**: NGINX and AWS ALB ingress controllers
- **TLS Termination**: With self-signed and cert-manager support
- **WebSocket Support**: For Storm UI real-time updates
- **Security Headers**: HSTS, CSP, X-Frame-Options, etc.

#### Configuration Management
- **values.schema.json**: Complete JSON schema for configuration validation
- **Production Values**: Pre-configured examples for different scenarios
- **Memory Auto-calculation**: Optimal JVM and worker memory settings

### Documentation
- **README.md**: Comprehensive guide with examples and troubleshooting
- **PRODUCTION.md**: Detailed production deployment guide
- **Individual Feature Guides**: For security, monitoring, HPA, and authentication

### Configuration Files
- `values-production.yaml`: Complete production configuration
- `values-production-security.yaml`: Security-focused configuration
- `values-production-monitoring.yaml`: Monitoring stack configuration
- `values-production-pdb.yaml`: High availability configuration
- `values-production-ingress.yaml`: Ingress with TLS configuration
- `values-production-hpa.yaml`: Autoscaling configuration
- `values-production-auth.yaml`: Authentication configuration

## [0.1.0] - Initial Release

### Added
- Basic Storm deployment with Nimbus, Supervisor, and UI
- Embedded Zookeeper support
- External Zookeeper configuration
- Basic resource management
- Persistent volume support
- ConfigMap-based Storm configuration