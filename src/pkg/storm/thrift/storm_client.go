package thrift

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ThriftStormClient implements ThriftClient using a connection pool
type ThriftStormClient struct {
	config *ThriftClientConfig
	pool   *ConnectionPool
	mu     sync.RWMutex
}

// NewThriftStormClient creates a new Thrift-based Storm client
func NewThriftStormClient(config *ThriftClientConfig) (*ThriftStormClient, error) {
	if config == nil {
		config = DefaultThriftClientConfig()
	}

	poolConfig := &ConnectionPoolConfig{
		MaxConnections:     10,
		MinIdleConnections: 2,
		MaxIdleTime:        5 * time.Minute,
		MaxLifetime:        30 * time.Minute,
		ClientConfig:       config,
	}

	pool, err := NewConnectionPool(poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &ThriftStormClient{
		config: config,
		pool:   pool,
	}, nil
}

// NewThriftStormClientWithPool creates a new client with a custom pool (for testing)
func NewThriftStormClientWithPool(config *ThriftClientConfig, pool *ConnectionPool) *ThriftStormClient {
	if config == nil {
		config = DefaultThriftClientConfig()
	}

	return &ThriftStormClient{
		config: config,
		pool:   pool,
	}
}

// Connect establishes connection to Storm cluster
func (c *ThriftStormClient) Connect(ctx context.Context) error {
	// Connection pool handles connection management
	// Just verify we can get a connection
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	return nil
}

// Close closes all connections
func (c *ThriftStormClient) Close() error {
	return c.pool.Close()
}

// IsConnected checks if client can connect
func (c *ThriftStormClient) IsConnected() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := c.pool.Get(ctx)
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

// withConnection executes a function with a connection from the pool
func (c *ThriftStormClient) withConnection(ctx context.Context, fn func(*NimbusClient) error) error {
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Execute with retry logic
	var lastErr error
	for i := 0; i < c.config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(c.config.RetryDelay * time.Duration(i))
		}

		err = fn(conn.client)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if connection is broken
		if !conn.IsValid() {
			conn.Invalidate()
			// Get a new connection for retry
			conn, err = c.pool.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to get new connection for retry: %w", err)
			}
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", c.config.MaxRetries, lastErr)
}

// SubmitTopology submits a topology to Storm
func (c *ThriftStormClient) SubmitTopology(ctx context.Context, name string, uploadedJarLocation string, jsonConf string, topology *StormTopology) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.SubmitTopology(ctx, name, uploadedJarLocation, jsonConf, topology)
	})
}

// KillTopology kills a running topology
func (c *ThriftStormClient) KillTopology(ctx context.Context, name string) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.KillTopology(ctx, name)
	})
}

// KillTopologyWithOpts kills a topology with options
func (c *ThriftStormClient) KillTopologyWithOpts(ctx context.Context, name string, options *KillOptions) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.KillTopologyWithOpts(ctx, name, options)
	})
}

// Activate activates a topology
func (c *ThriftStormClient) Activate(ctx context.Context, name string) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.Activate(ctx, name)
	})
}

// Deactivate deactivates a topology
func (c *ThriftStormClient) Deactivate(ctx context.Context, name string) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.Deactivate(ctx, name)
	})
}

// Rebalance rebalances a topology
func (c *ThriftStormClient) Rebalance(ctx context.Context, name string, options *RebalanceOptions) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.Rebalance(ctx, name, options)
	})
}

// GetTopologyInfo gets detailed topology information
func (c *ThriftStormClient) GetTopologyInfo(ctx context.Context, id string) (*TopologyInfo, error) {
	var result *TopologyInfo
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetTopologyInfo(ctx, id)
		return err
	})
	return result, err
}

// GetTopologyPageInfo gets topology page information
func (c *ThriftStormClient) GetTopologyPageInfo(ctx context.Context, id string, window string, includeSys bool) (*TopologyPageInfo, error) {
	var result *TopologyPageInfo
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetTopologyPageInfo(ctx, id, window, includeSys)
		return err
	})
	return result, err
}

// GetTopology gets topology structure
func (c *ThriftStormClient) GetTopology(ctx context.Context, id string) (*StormTopology, error) {
	var result *StormTopology
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetTopology(ctx, id)
		return err
	})
	return result, err
}

// GetUserTopology gets user topology structure
func (c *ThriftStormClient) GetUserTopology(ctx context.Context, id string) (*StormTopology, error) {
	var result *StormTopology
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetUserTopology(ctx, id)
		return err
	})
	return result, err
}

// GetTopologyHistory gets topology history
func (c *ThriftStormClient) GetTopologyHistory(ctx context.Context, user string) (*TopologyHistoryInfo, error) {
	var result *TopologyHistoryInfo
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetTopologyHistory(ctx, user)
		return err
	})
	return result, err
}

// GetClusterInfo gets cluster summary information
func (c *ThriftStormClient) GetClusterInfo(ctx context.Context) (*ClusterSummary, error) {
	var result *ClusterSummary
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetClusterInfo(ctx)
		return err
	})
	return result, err
}

// GetLeader gets the current Nimbus leader
func (c *ThriftStormClient) GetLeader(ctx context.Context) (*NimbusSummary, error) {
	var result *NimbusSummary
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetLeader(ctx)
		return err
	})
	return result, err
}

// IsTopologyNameAllowed checks if a topology name is allowed
func (c *ThriftStormClient) IsTopologyNameAllowed(ctx context.Context, name string) (bool, error) {
	var result bool
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.IsTopologyNameAllowed(ctx, name)
		return err
	})
	return result, err
}

// GetSupervisorPageInfo gets supervisor page information
func (c *ThriftStormClient) GetSupervisorPageInfo(ctx context.Context, id string, host string, includeSys bool) (*SupervisorPageInfo, error) {
	var result *SupervisorPageInfo
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetSupervisorPageInfo(ctx, id, host, includeSys)
		return err
	})
	return result, err
}

// GetComponentPageInfo gets component page information
func (c *ThriftStormClient) GetComponentPageInfo(ctx context.Context, topologyId string, componentId string, window string, includeSys bool) (*ComponentPageInfo, error) {
	var result *ComponentPageInfo
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetComponentPageInfo(ctx, topologyId, componentId, window, includeSys)
		return err
	})
	return result, err
}

// GetComponentPendingProfileActions gets pending profile actions for a component
func (c *ThriftStormClient) GetComponentPendingProfileActions(ctx context.Context, id string, componentId string, action ProfileAction) ([]*ProfileRequest, error) {
	var result []*ProfileRequest
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetComponentPendingProfileActions(ctx, id, componentId, action)
		return err
	})
	return result, err
}

// GetTopologyConf gets topology configuration
func (c *ThriftStormClient) GetTopologyConf(ctx context.Context, id string) (string, error) {
	var result string
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetTopologyConf(ctx, id)
		return err
	})
	return result, err
}

// GetOwnerResourceSummaries gets resource summaries for an owner
func (c *ThriftStormClient) GetOwnerResourceSummaries(ctx context.Context, owner string) ([]*OwnerResourceSummary, error) {
	var result []*OwnerResourceSummary
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetOwnerResourceSummaries(ctx, owner)
		return err
	})
	return result, err
}

// Debug enables/disables debug mode for a topology
func (c *ThriftStormClient) Debug(ctx context.Context, name string, component string, enable bool, samplingPercentage float64) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.Debug(ctx, name, component, enable, samplingPercentage)
	})
}

// SetWorkerProfiler sets worker profiler settings
func (c *ThriftStormClient) SetWorkerProfiler(ctx context.Context, id string, profileRequest *ProfileRequest) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.SetWorkerProfiler(ctx, id, profileRequest)
	})
}

// GetWorkerProfileActionExpiry gets worker profile action expiry
func (c *ThriftStormClient) GetWorkerProfileActionExpiry(ctx context.Context, id string, action ProfileAction) (map[string]int64, error) {
	// TODO: This method doesn't exist in the current Thrift definition
	// May need to be implemented differently or removed
	return nil, fmt.Errorf("GetWorkerProfileActionExpiry not implemented")
}

// GetTopologyLogConfig gets topology log configuration
func (c *ThriftStormClient) GetTopologyLogConfig(ctx context.Context, id string) (*LogConfig, error) {
	var result *LogConfig
	err := c.withConnection(ctx, func(client *NimbusClient) error {
		var err error
		result, err = client.GetLogConfig(ctx, id)
		return err
	})
	return result, err
}

// SetTopologyLogConfig sets topology log configuration
func (c *ThriftStormClient) SetTopologyLogConfig(ctx context.Context, id string, config *LogConfig) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.SetLogConfig(ctx, id, config)
	})
}

// SetLogConfig sets log configuration
func (c *ThriftStormClient) SetLogConfig(ctx context.Context, name string, config *LogConfig) error {
	return c.withConnection(ctx, func(client *NimbusClient) error {
		return client.SetLogConfig(ctx, name, config)
	})
}
