# Changelog

## [0.4.0] - Environment Variable Configuration

### Added
- **Environment Variable ConfigMap**: All Storm configuration is now managed through environment variables
- **Optional storm.yaml**: Support for custom storm.yaml via `cluster.stormYaml` option
- **Per-Component LOG_FORMAT**: Set logging format individually for each component
- **Migration Guide**: Comprehensive documentation for updating from previous versions

### Changed
- **Configuration Method**: Moved from inline environment variables to centralized ConfigMap
- **Image Tags**: Updated to use new Storm container version `2.8.1-17-jre`
- **Log Configuration**: Removed log4j2 XML from helm chart (now handled by container)

### Deprecated
- **cluster.logFormat**: Use component-level `extraEnvVars` with `LOG_FORMAT` instead
- **clusterConfig section**: Use `cluster.extraConfig` and component-specific `extraConfig` instead
- **Helper Functions**: Removed unused template helpers (renderClusterConfig, configToEnv, memoryConfig)

### Removed
- **Log4j2 ConfigMap entries**: No longer needed with new container
- **Unused helper functions**: Cleaned up template helpers that are no longer used

## [0.3.0] - Memory Management and Enhanced Features

### Added

#### Memory Configuration System
- **Auto Mode**: Automatic calculation of container resources based on worker requirements
- **Manual Mode**: Full control over memory settings for advanced users
- **Memory Helpers**: Template functions for consistent memory allocation
- **Validation**: Built-in checks to prevent memory misconfiguration

#### Enhanced Monitoring
- **Datadog Integration**: Full support with unified service tagging
- **Log Annotations**: Automatic log collection annotations for Datadog
- **JSON Logging**: Structured logging support for better log aggregation
- **Environment Variables**: DD_ENV, DD_SERVICE, DD_VERSION injection

#### Supervisor Improvements
- **Dynamic Pod IP**: Supervisors use pod IP for storm.local.hostname
- **Flexible Scaling**: Support for 1 worker per supervisor configurations
- **Resource Validation**: Automatic validation of memory settings

### Changed
- **Memory Configuration**: Replaced `autoMemory` with `memoryConfig` system
- **Default Supervisor Count**: Changed from 3 to 1 for development environments
- **HPA Configuration**: Updated to support new memory configuration

### Fixed
- **Supervisor Connection Issues**: Fixed "getLeader failed" errors
- **Assignment Synchronization**: Resolved getSupervisorAssignments failures
- **Helm Context**: Fixed template context passing for Datadog annotations

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