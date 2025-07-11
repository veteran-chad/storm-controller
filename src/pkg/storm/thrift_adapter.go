package storm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/veteran-chad/storm-controller/pkg/storm/thrift"
)

// ThriftAdapter adapts the Thrift client to implement the Storm Client interface
type ThriftAdapter struct {
	thriftClient thrift.ThriftClient
	nimbusHost   string
	nimbusPort   int
	uiHost       string
	uiPort       int
}

// NewThriftAdapter creates a new adapter that uses Thrift for Storm operations
func NewThriftAdapter(nimbusHost string, nimbusPort int, uiHost string, uiPort int, thriftConfig *thrift.ThriftClientConfig) (Client, error) {
	// Use provided config or create default
	if thriftConfig == nil {
		thriftConfig = &thrift.ThriftClientConfig{
			Host:              nimbusHost,
			Port:              nimbusPort,
			ConnectionTimeout: 30 * time.Second,
			RequestTimeout:    60 * time.Second,
			MaxRetries:        3,
			RetryDelay:        1 * time.Second,
		}
	} else {
		// Override host and port from parameters
		thriftConfig.Host = nimbusHost
		thriftConfig.Port = nimbusPort
	}

	thriftClient, err := thrift.NewThriftStormClient(thriftConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Thrift client: %w", err)
	}

	return &ThriftAdapter{
		thriftClient: thriftClient,
		nimbusHost:   nimbusHost,
		nimbusPort:   nimbusPort,
		uiHost:       uiHost,
		uiPort:       uiPort,
	}, nil
}

// SubmitTopology submits a new topology to the cluster using Thrift
func (ta *ThriftAdapter) SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error {
	// For now, we need the topology structure which requires parsing the JAR
	// This is a limitation - in practice, we'd need to either:
	// 1. Parse the JAR to extract the topology
	// 2. Use a different submission method
	// 3. Have the topology structure provided separately

	// For now, return an error indicating we need to use CLI for submission
	return fmt.Errorf("topology submission via Thrift requires topology structure - use CLI for now")
}

// KillTopology kills a running topology using Thrift
func (ta *ThriftAdapter) KillTopology(ctx context.Context, name string, waitSecs int) error {
	// Create kill options
	waitSecsInt32 := int32(waitSecs)
	killOpts := &thrift.KillOptions{
		WaitSecs: &waitSecsInt32,
	}

	return ta.thriftClient.KillTopologyWithOpts(ctx, name, killOpts)
}

// ActivateTopology activates a deactivated topology using Thrift
func (ta *ThriftAdapter) ActivateTopology(ctx context.Context, name string) error {
	return ta.thriftClient.Activate(ctx, name)
}

// DeactivateTopology deactivates an active topology using Thrift
func (ta *ThriftAdapter) DeactivateTopology(ctx context.Context, name string) error {
	return ta.thriftClient.Deactivate(ctx, name)
}

// RebalanceTopology rebalances a topology using Thrift
func (ta *ThriftAdapter) RebalanceTopology(ctx context.Context, name string, options *RebalanceOptions) error {
	// Convert options to Thrift format
	thriftOpts := &thrift.RebalanceOptions{}

	if options != nil {
		if options.WaitSecs > 0 {
			waitSecs := int32(options.WaitSecs)
			thriftOpts.WaitSecs = &waitSecs
		}
		if options.NumWorkers > 0 {
			numWorkers := int32(options.NumWorkers)
			thriftOpts.NumWorkers = &numWorkers
		}
		if len(options.NumExecutors) > 0 {
			thriftOpts.NumExecutors = make(map[string]int32)
			for k, v := range options.NumExecutors {
				thriftOpts.NumExecutors[k] = int32(v)
			}
		}
		// Note: Configuration changes during rebalance might not be supported
	}

	return ta.thriftClient.Rebalance(ctx, name, thriftOpts)
}

// GetTopology gets detailed information about a topology using Thrift
func (ta *ThriftAdapter) GetTopology(ctx context.Context, name string) (*TopologyInfo, error) {
	// First, we need to find the topology ID by name
	// The Thrift API typically works with topology IDs, not names
	topologies, err := ta.ListTopologies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list topologies: %w", err)
	}

	var topologyID string
	for _, t := range topologies {
		if t.Name == name {
			topologyID = t.ID
			break
		}
	}

	if topologyID == "" {
		return nil, fmt.Errorf("topology '%s' not found", name)
	}

	// Get detailed topology info
	thriftInfo, err := ta.thriftClient.GetTopologyInfo(ctx, topologyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topology info: %w", err)
	}

	// Get topology configuration
	configJSON, err := ta.thriftClient.GetTopologyConf(ctx, topologyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topology config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		// If unmarshal fails, just use empty config
		config = make(map[string]interface{})
	}

	// Convert Thrift info to our format
	info := &TopologyInfo{
		ID:            thriftInfo.ID,
		Name:          thriftInfo.Name,
		Status:        thriftInfo.Status,
		UptimeSeconds: int(thriftInfo.UptimeSecs),
		Workers:       len(thriftInfo.Executors), // Approximate
		Executors:     len(thriftInfo.Executors),
		Tasks:         len(thriftInfo.Executors), // Approximate
		Configuration: config,
		Stats:         make(map[string]interface{}), // Would need additional calls
	}

	if thriftInfo.ReplicationCount != nil {
		info.ReplicationCount = int(*thriftInfo.ReplicationCount)
	}

	return info, nil
}

// ListTopologies lists all topologies in the cluster using Thrift
func (ta *ThriftAdapter) ListTopologies(ctx context.Context) ([]TopologySummary, error) {
	clusterInfo, err := ta.thriftClient.GetClusterInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}

	summaries := make([]TopologySummary, 0, len(clusterInfo.Topologies))

	for _, t := range clusterInfo.Topologies {
		summary := TopologySummary{
			ID:            t.ID,
			Name:          t.Name,
			Status:        t.Status,
			UptimeSeconds: int(t.UptimeSecs),
			Workers:       int(t.NumWorkers),
			Executors:     int(t.NumExecutors),
			Tasks:         int(t.NumTasks),
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetClusterInfo gets information about the Storm cluster using Thrift
func (ta *ThriftAdapter) GetClusterInfo(ctx context.Context) (*ClusterSummary, error) {
	thriftInfo, err := ta.thriftClient.GetClusterInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}

	// Get leader info
	leader, err := ta.thriftClient.GetLeader(ctx)
	if err != nil {
		// Non-fatal, just log
		leader = nil
	}

	// Calculate slots
	totalSlots := 0
	usedSlots := 0

	for _, supervisor := range thriftInfo.Supervisors {
		totalSlots += int(supervisor.NumWorkers)
		usedSlots += int(supervisor.NumUsedWorkers)
	}

	summary := &ClusterSummary{
		Supervisors: len(thriftInfo.Supervisors),
		UsedSlots:   usedSlots,
		TotalSlots:  totalSlots,
		FreeSlots:   totalSlots - usedSlots,
		Topologies:  len(thriftInfo.Topologies),
		NimbusHosts: make([]string, 0),
	}

	// Add Nimbus hosts
	for _, nimbus := range thriftInfo.Nimbuses {
		summary.NimbusHosts = append(summary.NimbusHosts, nimbus.Host)
		if nimbus.IsLeader {
			summary.NimbusLeader = nimbus.Host
			summary.NimbusUptime = int(nimbus.UptimeSecs)
		}
	}

	// If we got leader info separately, use it
	if leader != nil && summary.NimbusLeader == "" {
		summary.NimbusLeader = leader.Host
		summary.NimbusUptime = int(leader.UptimeSecs)
	}

	return summary, nil
}

// GetClusterConfiguration gets the cluster configuration using Thrift
func (ta *ThriftAdapter) GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error) {
	// This would require a specific Thrift call to get cluster config
	// For now, return empty config
	return make(map[string]interface{}), nil
}

// UploadJar uploads a JAR file to the cluster
func (ta *ThriftAdapter) UploadJar(ctx context.Context, jarPath string, jarData []byte) error {
	// Thrift doesn't directly support JAR upload
	// This is typically done through the REST API or file system
	return fmt.Errorf("JAR upload not supported via Thrift - use REST API or shared filesystem")
}

// DownloadJar downloads a JAR file from a URL
func (ta *ThriftAdapter) DownloadJar(ctx context.Context, url string) ([]byte, error) {
	// This is not a Storm-specific operation
	// Reuse the REST client implementation
	restClient := NewClient(ta.nimbusHost, ta.nimbusPort, ta.uiHost, ta.uiPort)
	return restClient.DownloadJar(ctx, url)
}

// Close closes the Thrift client connection
func (ta *ThriftAdapter) Close() error {
	return ta.thriftClient.Close()
}
