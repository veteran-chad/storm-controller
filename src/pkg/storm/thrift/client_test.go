package thrift

import (
	"context"
	"testing"
	"time"
)

func TestDefaultThriftClientConfig(t *testing.T) {
	config := DefaultThriftClientConfig()

	if config.Host != "localhost" {
		t.Errorf("Expected host to be localhost, got %s", config.Host)
	}

	if config.Port != 6627 {
		t.Errorf("Expected port to be 6627, got %d", config.Port)
	}

	if config.ConnectionTimeout != 30*time.Second {
		t.Errorf("Expected connection timeout to be 30s, got %v", config.ConnectionTimeout)
	}

	if config.RequestTimeout != 60*time.Second {
		t.Errorf("Expected request timeout to be 60s, got %v", config.RequestTimeout)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries to be 3, got %d", config.MaxRetries)
	}

	if config.RetryDelay != 1*time.Second {
		t.Errorf("Expected retry delay to be 1s, got %v", config.RetryDelay)
	}

	if config.UseTLS {
		t.Error("Expected UseTLS to be false by default")
	}
}

func TestThriftClientConfigWithCustomValues(t *testing.T) {
	config := &ThriftClientConfig{
		Host:              "storm-nimbus",
		Port:              7627,
		ConnectionTimeout: 10 * time.Second,
		RequestTimeout:    30 * time.Second,
		MaxRetries:        5,
		RetryDelay:        2 * time.Second,
		UseTLS:            true,
		CertFile:          "cert.pem",
		KeyFile:           "key.pem",
		CAFile:            "ca.pem",
	}

	if config.Host != "storm-nimbus" {
		t.Errorf("Expected host to be storm-nimbus, got %s", config.Host)
	}

	if config.Port != 7627 {
		t.Errorf("Expected port to be 7627, got %d", config.Port)
	}

	if !config.UseTLS {
		t.Error("Expected UseTLS to be true")
	}

	if config.CertFile != "cert.pem" {
		t.Errorf("Expected cert file to be cert.pem, got %s", config.CertFile)
	}
}

// MockThriftClient implements ThriftClient interface for testing
type MockThriftClient struct {
	connected      bool
	connectError   error
	topologies     map[string]*TopologyInfo
	clusterSummary *ClusterSummary
	submitCalls    []submitCall
	killCalls      []string
}

type submitCall struct {
	name     string
	jarPath  string
	jsonConf string
	topology *StormTopology
}

func NewMockThriftClient() *MockThriftClient {
	return &MockThriftClient{
		topologies:  make(map[string]*TopologyInfo),
		submitCalls: []submitCall{},
		killCalls:   []string{},
	}
}

func (m *MockThriftClient) Connect(ctx context.Context) error {
	if m.connectError != nil {
		return m.connectError
	}
	m.connected = true
	return nil
}

func (m *MockThriftClient) Close() error {
	m.connected = false
	return nil
}

func (m *MockThriftClient) IsConnected() bool {
	return m.connected
}

func (m *MockThriftClient) SubmitTopology(ctx context.Context, name string, uploadedJarLocation string, jsonConf string, topology *StormTopology) error {
	m.submitCalls = append(m.submitCalls, submitCall{
		name:     name,
		jarPath:  uploadedJarLocation,
		jsonConf: jsonConf,
		topology: topology,
	})

	// Add to topologies
	m.topologies[name] = &TopologyInfo{
		ID:         name,
		Name:       name,
		Status:     "ACTIVE",
		UptimeSecs: 0,
		Executors:  []*ExecutorSummary{},
		Errors:     make(map[string][]*ErrorInfo),
	}

	return nil
}

func (m *MockThriftClient) KillTopology(ctx context.Context, name string) error {
	m.killCalls = append(m.killCalls, name)
	delete(m.topologies, name)
	return nil
}

func (m *MockThriftClient) KillTopologyWithOpts(ctx context.Context, name string, options *KillOptions) error {
	return m.KillTopology(ctx, name)
}

func (m *MockThriftClient) Activate(ctx context.Context, name string) error {
	if info, exists := m.topologies[name]; exists {
		info.Status = "ACTIVE"
	}
	return nil
}

func (m *MockThriftClient) Deactivate(ctx context.Context, name string) error {
	if info, exists := m.topologies[name]; exists {
		info.Status = "INACTIVE"
	}
	return nil
}

func (m *MockThriftClient) Rebalance(ctx context.Context, name string, options *RebalanceOptions) error {
	return nil
}

func (m *MockThriftClient) GetTopologyInfo(ctx context.Context, id string) (*TopologyInfo, error) {
	if info, exists := m.topologies[id]; exists {
		return info, nil
	}
	return nil, nil
}

func (m *MockThriftClient) GetTopologyPageInfo(ctx context.Context, id string, window string, includeSys bool) (*TopologyPageInfo, error) {
	return nil, nil
}

func (m *MockThriftClient) GetTopology(ctx context.Context, id string) (*StormTopology, error) {
	return nil, nil
}

func (m *MockThriftClient) GetUserTopology(ctx context.Context, id string) (*StormTopology, error) {
	return nil, nil
}

func (m *MockThriftClient) GetTopologyHistory(ctx context.Context, user string) (*TopologyHistoryInfo, error) {
	return nil, nil
}

func (m *MockThriftClient) GetClusterInfo(ctx context.Context) (*ClusterSummary, error) {
	if m.clusterSummary != nil {
		return m.clusterSummary, nil
	}

	// Return default cluster summary
	return &ClusterSummary{
		Supervisors: []*SupervisorSummary{
			{SupervisorID: "supervisor-1", Host: "host1", UptimeSecs: 3600, NumWorkers: 4, NumUsedWorkers: 1, Version: "2.0.0"},
			{SupervisorID: "supervisor-2", Host: "host2", UptimeSecs: 3600, NumWorkers: 4, NumUsedWorkers: 1, Version: "2.0.0"},
			{SupervisorID: "supervisor-3", Host: "host3", UptimeSecs: 3600, NumWorkers: 4, NumUsedWorkers: 2, Version: "2.0.0"},
		},
		Nimbuses: []*NimbusSummary{
			{Host: "nimbus-0", Port: 6627, UptimeSecs: 3600, IsLeader: true},
		},
		Topologies: []*TopologySummary{},
	}, nil
}

func (m *MockThriftClient) GetLeader(ctx context.Context) (*NimbusSummary, error) {
	return &NimbusSummary{
		Host:       "nimbus-0",
		Port:       6627,
		UptimeSecs: 3600,
		IsLeader:   true,
	}, nil
}

func (m *MockThriftClient) IsTopologyNameAllowed(ctx context.Context, name string) (bool, error) {
	// Check if topology already exists
	_, exists := m.topologies[name]
	return !exists, nil
}

func (m *MockThriftClient) GetSupervisorPageInfo(ctx context.Context, id string, host string, includeSys bool) (*SupervisorPageInfo, error) {
	return nil, nil
}

func (m *MockThriftClient) GetComponentPageInfo(ctx context.Context, topologyId string, componentId string, window string, includeSys bool) (*ComponentPageInfo, error) {
	return nil, nil
}

func (m *MockThriftClient) GetComponentPendingProfileActions(ctx context.Context, id string, componentId string, action ProfileAction) ([]*ProfileRequest, error) {
	return nil, nil
}

func (m *MockThriftClient) GetTopologyConf(ctx context.Context, id string) (string, error) {
	return "{}", nil
}

func (m *MockThriftClient) GetOwnerResourceSummaries(ctx context.Context, owner string) ([]*OwnerResourceSummary, error) {
	return nil, nil
}

func (m *MockThriftClient) Debug(ctx context.Context, name string, component string, enable bool, samplingPercentage float64) error {
	return nil
}

func (m *MockThriftClient) SetWorkerProfiler(ctx context.Context, id string, profileRequest *ProfileRequest) error {
	return nil
}

func (m *MockThriftClient) GetWorkerProfileActionExpiry(ctx context.Context, id string, action ProfileAction) (map[string]int64, error) {
	return nil, nil
}

func (m *MockThriftClient) GetTopologyLogConfig(ctx context.Context, id string) (*LogConfig, error) {
	return nil, nil
}

func (m *MockThriftClient) SetTopologyLogConfig(ctx context.Context, id string, config *LogConfig) error {
	return nil
}

func (m *MockThriftClient) SetLogConfig(ctx context.Context, name string, config *LogConfig) error {
	return nil
}

// Test using the mock client
func TestMockThriftClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockThriftClient()

	// Test connection
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Expected client to be connected")
	}

	// Test submit topology
	err := client.SubmitTopology(ctx, "test-topology", "/tmp/test.jar", "{}", nil)
	if err != nil {
		t.Fatalf("Failed to submit topology: %v", err)
	}

	if len(client.submitCalls) != 1 {
		t.Errorf("Expected 1 submit call, got %d", len(client.submitCalls))
	}

	// Test get topology info
	info, err := client.GetTopologyInfo(ctx, "test-topology")
	if err != nil {
		t.Fatalf("Failed to get topology info: %v", err)
	}

	if info == nil || info.Name != "test-topology" {
		t.Error("Expected to get topology info for test-topology")
	}

	// Test kill topology
	err = client.KillTopology(ctx, "test-topology")
	if err != nil {
		t.Fatalf("Failed to kill topology: %v", err)
	}

	if len(client.killCalls) != 1 {
		t.Errorf("Expected 1 kill call, got %d", len(client.killCalls))
	}

	// Test close
	err = client.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	if client.IsConnected() {
		t.Error("Expected client to be disconnected")
	}
}
