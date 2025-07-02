/*
Copyright 2025 The Apache Software Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the interface for interacting with a Storm cluster
type Client interface {
	// Topology operations
	SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error
	KillTopology(ctx context.Context, name string, waitSecs int) error
	ActivateTopology(ctx context.Context, name string) error
	DeactivateTopology(ctx context.Context, name string) error
	RebalanceTopology(ctx context.Context, name string, options *RebalanceOptions) error
	GetTopology(ctx context.Context, name string) (*TopologyInfo, error)
	ListTopologies(ctx context.Context) ([]TopologySummary, error)

	// Cluster operations
	GetClusterInfo(ctx context.Context) (*ClusterSummary, error)
	GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error)

	// JAR operations
	UploadJar(ctx context.Context, jarPath string, jarData []byte) error
	DownloadJar(ctx context.Context, url string) ([]byte, error)
}

// client implements the Storm Client interface
type client struct {
	nimbusHost string
	nimbusPort int
	uiHost     string
	uiPort     int
	httpClient *http.Client
	useREST    bool // Use REST API as fallback
}

// NewClient creates a new Storm client
func NewClient(nimbusHost string, nimbusPort int, uiHost string, uiPort int) Client {
	return &client{
		nimbusHost: nimbusHost,
		nimbusPort: nimbusPort,
		uiHost:     uiHost,
		uiPort:     uiPort,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		useREST: true, // Start with REST, fallback to Thrift if needed
	}
}

// RebalanceOptions contains options for rebalancing a topology
type RebalanceOptions struct {
	WaitSecs        int               `json:"wait_secs,omitempty"`
	NumWorkers      int               `json:"num_workers,omitempty"`
	NumExecutors    map[string]int    `json:"num_executors,omitempty"`
	Configuration   map[string]string `json:"configuration,omitempty"`
}

// TopologyInfo contains detailed information about a topology
type TopologyInfo struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Status          string                 `json:"status"`
	UptimeSeconds   int                    `json:"uptime_secs"`
	Workers         int                    `json:"workers_total"`
	Executors       int                    `json:"executors_total"`
	Tasks           int                    `json:"tasks_total"`
	ReplicationCount int                   `json:"replication_count"`
	Configuration   map[string]interface{} `json:"configuration"`
	Stats           map[string]interface{} `json:"stats"`
}

// TopologySummary contains summary information about a topology
type TopologySummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	UptimeSeconds int    `json:"uptime_secs"`
	Workers       int    `json:"workers_total"`
	Executors     int    `json:"executors_total"`
	Tasks         int    `json:"tasks_total"`
}

// ClusterSummary contains information about the Storm cluster
type ClusterSummary struct {
	Supervisors      int                    `json:"supervisors"`
	UsedSlots        int                    `json:"slots_used"`
	TotalSlots       int                    `json:"slots_total"`
	FreeSlots        int                    `json:"slots_free"`
	NimbusUptime     int                    `json:"nimbus_uptime_secs"`
	Topologies       int                    `json:"topologies"`
	NimbusLeader     string                 `json:"nimbus_leader"`
	NimbusHosts      []string               `json:"nimbus_hosts"`
	Configuration    map[string]interface{} `json:"configuration"`
}

// REST API implementation

func (c *client) restURL(path string) string {
	return fmt.Sprintf("http://%s:%d/api/v1%s", c.uiHost, c.uiPort, path)
}

func (c *client) doREST(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.restURL(path), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// SubmitTopology submits a new topology to the cluster
func (c *client) SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error {
	// For now, we'll use storm CLI via exec
	// In a real implementation, we would use Thrift API
	// This is a placeholder that returns an error
	return fmt.Errorf("topology submission via API not yet implemented - use storm CLI")
}

// KillTopology kills a running topology
func (c *client) KillTopology(ctx context.Context, name string, waitSecs int) error {
	path := fmt.Sprintf("/topology/%s/kill/%d", name, waitSecs)
	_, err := c.doREST(ctx, http.MethodPost, path, nil)
	return err
}

// ActivateTopology activates a deactivated topology
func (c *client) ActivateTopology(ctx context.Context, name string) error {
	path := fmt.Sprintf("/topology/%s/activate", name)
	_, err := c.doREST(ctx, http.MethodPost, path, nil)
	return err
}

// DeactivateTopology deactivates an active topology
func (c *client) DeactivateTopology(ctx context.Context, name string) error {
	path := fmt.Sprintf("/topology/%s/deactivate", name)
	_, err := c.doREST(ctx, http.MethodPost, path, nil)
	return err
}

// RebalanceTopology rebalances a topology
func (c *client) RebalanceTopology(ctx context.Context, name string, options *RebalanceOptions) error {
	path := fmt.Sprintf("/topology/%s/rebalance", name)
	_, err := c.doREST(ctx, http.MethodPost, path, options)
	return err
}

// GetTopology gets detailed information about a topology
func (c *client) GetTopology(ctx context.Context, name string) (*TopologyInfo, error) {
	// First try by name
	path := fmt.Sprintf("/topology/%s", name)
	data, err := c.doREST(ctx, http.MethodGet, path, nil)
	if err != nil {
		// If failed, try to find by listing all topologies
		topologies, listErr := c.ListTopologies(ctx)
		if listErr != nil {
			return nil, err // Return original error
		}
		
		for _, t := range topologies {
			if t.Name == name {
				// Try again with ID
				path = fmt.Sprintf("/topology/%s", t.ID)
				data, err = c.doREST(ctx, http.MethodGet, path, nil)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		
		if err != nil {
			return nil, fmt.Errorf("topology '%s' not found", name)
		}
	}

	var info TopologyInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topology info: %w", err)
	}

	return &info, nil
}

// ListTopologies lists all topologies in the cluster
func (c *client) ListTopologies(ctx context.Context) ([]TopologySummary, error) {
	data, err := c.doREST(ctx, http.MethodGet, "/topology/summary", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Topologies []TopologySummary `json:"topologies"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topologies: %w", err)
	}

	return resp.Topologies, nil
}

// GetClusterInfo gets information about the Storm cluster
func (c *client) GetClusterInfo(ctx context.Context) (*ClusterSummary, error) {
	data, err := c.doREST(ctx, http.MethodGet, "/cluster/summary", nil)
	if err != nil {
		return nil, err
	}

	var summary ClusterSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster summary: %w", err)
	}

	return &summary, nil
}

// GetClusterConfiguration gets the cluster configuration
func (c *client) GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error) {
	data, err := c.doREST(ctx, http.MethodGet, "/cluster/configuration", nil)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster configuration: %w", err)
	}

	return config, nil
}

// UploadJar uploads a JAR file to the cluster
func (c *client) UploadJar(ctx context.Context, jarPath string, jarData []byte) error {
	// This would typically use the Storm file upload API
	// For now, we assume JARs are accessible via URLs
	return fmt.Errorf("JAR upload not implemented - use HTTP URLs for JAR files")
}

// DownloadJar downloads a JAR file from a URL
func (c *client) DownloadJar(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download JAR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download JAR: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read JAR data: %w", err)
	}

	return data, nil
}