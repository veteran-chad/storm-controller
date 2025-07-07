package storm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/veteran-chad/storm-controller/pkg/storm/thrift"
)

// ClientType represents the type of Storm client to use
type ClientType string

const (
	// ClientTypeCLI uses Storm CLI commands (legacy)
	ClientTypeCLI ClientType = "cli"
	// ClientTypeREST uses Storm REST API
	ClientTypeREST ClientType = "rest"
	// ClientTypeThrift uses Storm Thrift API (recommended)
	ClientTypeThrift ClientType = "thrift"
)

// ClientConfig holds configuration for creating Storm clients
type ClientConfig struct {
	// Type of client to create
	Type ClientType

	// Common configuration
	NimbusHost string
	NimbusPort int
	UIHost     string
	UIPort     int

	// Thrift-specific configuration
	ThriftConfig *thrift.ThriftClientConfig

	// CLI-specific configuration
	StormBinary string // Path to storm binary
	StormImage  string // Docker image for storm commands
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() *ClientConfig {
	// Check environment variables for configuration
	clientType := os.Getenv("STORM_CLIENT_TYPE")
	if clientType == "" {
		clientType = string(ClientTypeThrift) // Default to Thrift
	}

	nimbusHost := os.Getenv("STORM_NIMBUS_HOST")
	if nimbusHost == "" {
		nimbusHost = "localhost"
	}

	nimbusPort := 6627
	if port := os.Getenv("STORM_NIMBUS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			nimbusPort = p
		}
	}

	uiHost := os.Getenv("STORM_UI_HOST")
	if uiHost == "" {
		uiHost = nimbusHost
	}

	uiPort := 8080
	if port := os.Getenv("STORM_UI_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			uiPort = p
		}
	}

	return &ClientConfig{
		Type:       ClientType(clientType),
		NimbusHost: nimbusHost,
		NimbusPort: nimbusPort,
		UIHost:     uiHost,
		UIPort:     uiPort,
		ThriftConfig: &thrift.ThriftClientConfig{
			Host:              nimbusHost,
			Port:              nimbusPort,
			ConnectionTimeout: 30 * time.Second,
			RequestTimeout:    60 * time.Second,
			MaxRetries:        3,
			RetryDelay:        1 * time.Second,
		},
		StormBinary: "storm",
		StormImage:  "storm:latest",
	}
}

// NewClientFromConfig creates a new Storm client based on configuration
func NewClientFromConfig(config *ClientConfig) (Client, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	switch config.Type {
	case ClientTypeCLI:
		// CLI client not implemented - use REST for now
		return NewClient(config.NimbusHost, config.NimbusPort, config.UIHost, config.UIPort), nil

	case ClientTypeREST:
		return NewClient(config.NimbusHost, config.NimbusPort, config.UIHost, config.UIPort), nil

	case ClientTypeThrift:
		return NewThriftAdapter(
			config.NimbusHost,
			config.NimbusPort,
			config.UIHost,
			config.UIPort,
			config.ThriftConfig,
		)

	default:
		return nil, fmt.Errorf("unknown client type: %s", config.Type)
	}
}

// Feature flags for gradual rollout
type FeatureFlags struct {
	// UseThriftForStatus enables Thrift for topology status checks
	UseThriftForStatus bool
	// UseThriftForKill enables Thrift for topology deletion
	UseThriftForKill bool
	// UseThriftForLifecycle enables Thrift for activate/deactivate/rebalance
	UseThriftForLifecycle bool
	// UseThriftForClusterInfo enables Thrift for cluster information
	UseThriftForClusterInfo bool
}

// DefaultFeatureFlags returns default feature flags
func DefaultFeatureFlags() *FeatureFlags {
	return &FeatureFlags{
		UseThriftForStatus:      getEnvBool("STORM_THRIFT_STATUS", true),
		UseThriftForKill:        getEnvBool("STORM_THRIFT_KILL", true),
		UseThriftForLifecycle:   getEnvBool("STORM_THRIFT_LIFECYCLE", true),
		UseThriftForClusterInfo: getEnvBool("STORM_THRIFT_CLUSTER_INFO", true),
	}
}

// HybridClient uses multiple client types based on feature flags
type HybridClient struct {
	primary  Client
	fallback Client
	flags    *FeatureFlags
}

// NewHybridClient creates a client that can use different backends for different operations
func NewHybridClient(primary, fallback Client, flags *FeatureFlags) Client {
	if flags == nil {
		flags = DefaultFeatureFlags()
	}
	return &HybridClient{
		primary:  primary,
		fallback: fallback,
		flags:    flags,
	}
}

// Implement Client interface with feature flag checks

func (h *HybridClient) SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error {
	// Topology submission still uses fallback (CLI) as it requires JAR parsing
	return h.fallback.SubmitTopology(ctx, name, jarPath, mainClass, args, config)
}

func (h *HybridClient) KillTopology(ctx context.Context, name string, waitSecs int) error {
	if h.flags.UseThriftForKill {
		return h.primary.KillTopology(ctx, name, waitSecs)
	}
	return h.fallback.KillTopology(ctx, name, waitSecs)
}

func (h *HybridClient) ActivateTopology(ctx context.Context, name string) error {
	if h.flags.UseThriftForLifecycle {
		return h.primary.ActivateTopology(ctx, name)
	}
	return h.fallback.ActivateTopology(ctx, name)
}

func (h *HybridClient) DeactivateTopology(ctx context.Context, name string) error {
	if h.flags.UseThriftForLifecycle {
		return h.primary.DeactivateTopology(ctx, name)
	}
	return h.fallback.DeactivateTopology(ctx, name)
}

func (h *HybridClient) RebalanceTopology(ctx context.Context, name string, options *RebalanceOptions) error {
	if h.flags.UseThriftForLifecycle {
		return h.primary.RebalanceTopology(ctx, name, options)
	}
	return h.fallback.RebalanceTopology(ctx, name, options)
}

func (h *HybridClient) GetTopology(ctx context.Context, name string) (*TopologyInfo, error) {
	if h.flags.UseThriftForStatus {
		return h.primary.GetTopology(ctx, name)
	}
	return h.fallback.GetTopology(ctx, name)
}

func (h *HybridClient) ListTopologies(ctx context.Context) ([]TopologySummary, error) {
	if h.flags.UseThriftForStatus {
		return h.primary.ListTopologies(ctx)
	}
	return h.fallback.ListTopologies(ctx)
}

func (h *HybridClient) GetClusterInfo(ctx context.Context) (*ClusterSummary, error) {
	if h.flags.UseThriftForClusterInfo {
		return h.primary.GetClusterInfo(ctx)
	}
	return h.fallback.GetClusterInfo(ctx)
}

func (h *HybridClient) GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error) {
	if h.flags.UseThriftForClusterInfo {
		return h.primary.GetClusterConfiguration(ctx)
	}
	return h.fallback.GetClusterConfiguration(ctx)
}

func (h *HybridClient) UploadJar(ctx context.Context, jarPath string, jarData []byte) error {
	// JAR operations use fallback
	return h.fallback.UploadJar(ctx, jarPath, jarData)
}

func (h *HybridClient) DownloadJar(ctx context.Context, url string) ([]byte, error) {
	// JAR operations use fallback
	return h.fallback.DownloadJar(ctx, url)
}

// Helper function to parse boolean environment variables
func getEnvBool(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}
