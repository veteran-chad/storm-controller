package thrift

import (
	"context"
	"time"
)

// ThriftClient defines the interface for Storm Thrift client operations
type ThriftClient interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool

	// Topology operations
	SubmitTopology(ctx context.Context, name string, uploadedJarLocation string, jsonConf string, topology *StormTopology) error
	KillTopology(ctx context.Context, name string) error
	KillTopologyWithOpts(ctx context.Context, name string, options *KillOptions) error
	Activate(ctx context.Context, name string) error
	Deactivate(ctx context.Context, name string) error
	Rebalance(ctx context.Context, name string, options *RebalanceOptions) error

	// Topology information
	GetTopologyInfo(ctx context.Context, id string) (*TopologyInfo, error)
	GetTopologyPageInfo(ctx context.Context, id string, window string, includeSys bool) (*TopologyPageInfo, error)
	GetTopology(ctx context.Context, id string) (*StormTopology, error)
	GetUserTopology(ctx context.Context, id string) (*StormTopology, error)
	GetTopologyHistory(ctx context.Context, user string) (*TopologyHistoryInfo, error)

	// Cluster information
	GetClusterInfo(ctx context.Context) (*ClusterSummary, error)
	GetLeader(ctx context.Context) (*NimbusSummary, error)
	IsTopologyNameAllowed(ctx context.Context, name string) (bool, error)

	// Supervisor operations
	GetSupervisorPageInfo(ctx context.Context, id string, host string, includeSys bool) (*SupervisorPageInfo, error)

	// Component operations
	GetComponentPageInfo(ctx context.Context, topologyId string, componentId string, window string, includeSys bool) (*ComponentPageInfo, error)
	GetComponentPendingProfileActions(ctx context.Context, id string, componentId string, action ProfileAction) ([]*ProfileRequest, error)

	// Configuration
	GetTopologyConf(ctx context.Context, id string) (string, error)

	// Metrics
	GetOwnerResourceSummaries(ctx context.Context, owner string) ([]*OwnerResourceSummary, error)

	// Debug operations
	Debug(ctx context.Context, name string, component string, enable bool, samplingPercentage float64) error
	SetWorkerProfiler(ctx context.Context, id string, profileRequest *ProfileRequest) error
	GetWorkerProfileActionExpiry(ctx context.Context, id string, action ProfileAction) (map[string]int64, error)

	// Log operations
	GetTopologyLogConfig(ctx context.Context, id string) (*LogConfig, error)
	SetTopologyLogConfig(ctx context.Context, id string, config *LogConfig) error
	SetLogConfig(ctx context.Context, name string, config *LogConfig) error
}

// ThriftClientConfig holds configuration for the Thrift client
type ThriftClientConfig struct {
	// Nimbus host address
	Host string
	// Nimbus port (default: 6627)
	Port int
	// Connection timeout
	ConnectionTimeout time.Duration
	// Request timeout
	RequestTimeout time.Duration
	// Maximum retry attempts
	MaxRetries int
	// Retry delay
	RetryDelay time.Duration
	// TLS configuration (optional)
	UseTLS   bool
	CertFile string
	KeyFile  string
	CAFile   string
}

// DefaultThriftClientConfig returns default configuration
func DefaultThriftClientConfig() *ThriftClientConfig {
	return &ThriftClientConfig{
		Host:              "localhost",
		Port:              6627,
		ConnectionTimeout: 30 * time.Second,
		RequestTimeout:    60 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		UseTLS:            false,
	}
}
