/*
Copyright 2025 The Apache Software Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/veteran-chad/storm-controller/pkg/storm/thrift"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClientManager manages Storm client connections dynamically based on cluster availability
type ClientManager interface {
	// GetClient returns the current Storm client if available
	GetClient() (Client, error)
	// UpdateClient updates the Storm client configuration
	UpdateClient(config *ClientConfig) error
	// RemoveClient removes the current Storm client
	RemoveClient()
	// HasClient returns true if a client is currently configured
	HasClient() bool
}

// DynamicClientManager implements ClientManager with thread-safe operations
type DynamicClientManager struct {
	mu            sync.RWMutex
	client        Client
	config        *ClientConfig
	lastConnected time.Time
}

// NewClientManager creates a new client manager
func NewClientManager() ClientManager {
	return &DynamicClientManager{}
}

// GetClient returns the current Storm client if available
func (m *DynamicClientManager) GetClient() (Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.client == nil {
		return nil, fmt.Errorf("no Storm client available - waiting for StormCluster resource")
	}
	return m.client, nil
}

// UpdateClient updates the Storm client configuration
func (m *DynamicClientManager) UpdateClient(config *ClientConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If config hasn't changed, don't recreate client
	if m.config != nil && m.config.Equals(config) && m.client != nil {
		return nil
	}

	// Close existing client if any
	if m.client != nil {
		// TODO: Add Close() method to Client interface if needed
		m.client = nil
	}

	// Create new client
	ctx := context.Background()
	log := log.FromContext(ctx)

	log.Info("Creating new Storm client",
		"nimbusHost", config.NimbusHost,
		"nimbusPort", config.NimbusPort)

	client, err := NewClientFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Storm client: %w", err)
	}

	m.client = client
	m.config = config.Copy()
	m.lastConnected = time.Now()

	log.Info("Successfully created Storm client")
	return nil
}

// RemoveClient removes the current Storm client
func (m *DynamicClientManager) RemoveClient() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		// TODO: Add Close() method to Client interface if needed
		m.client = nil
		m.config = nil
	}
}

// HasClient returns true if a client is currently configured
func (m *DynamicClientManager) HasClient() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client != nil
}

// Copy creates a deep copy of ClientConfig
func (c *ClientConfig) Copy() *ClientConfig {
	if c == nil {
		return nil
	}

	copy := &ClientConfig{
		Type:        c.Type,
		NimbusHost:  c.NimbusHost,
		NimbusPort:  c.NimbusPort,
		UIHost:      c.UIHost,
		UIPort:      c.UIPort,
		StormBinary: c.StormBinary,
		StormImage:  c.StormImage,
	}

	// Deep copy ThriftConfig if present
	if c.ThriftConfig != nil {
		copy.ThriftConfig = &thrift.ThriftClientConfig{
			Host:              c.ThriftConfig.Host,
			Port:              c.ThriftConfig.Port,
			ConnectionTimeout: c.ThriftConfig.ConnectionTimeout,
			RequestTimeout:    c.ThriftConfig.RequestTimeout,
			MaxRetries:        c.ThriftConfig.MaxRetries,
			RetryDelay:        c.ThriftConfig.RetryDelay,
		}
	}

	return copy
}

// Equals compares two ClientConfig instances
func (c *ClientConfig) Equals(other *ClientConfig) bool {
	if c == nil || other == nil {
		return c == other
	}

	// Basic fields comparison
	if c.Type != other.Type ||
		c.NimbusHost != other.NimbusHost ||
		c.NimbusPort != other.NimbusPort ||
		c.UIHost != other.UIHost ||
		c.UIPort != other.UIPort ||
		c.StormBinary != other.StormBinary ||
		c.StormImage != other.StormImage {
		return false
	}

	// ThriftConfig comparison
	if (c.ThriftConfig == nil) != (other.ThriftConfig == nil) {
		return false
	}

	if c.ThriftConfig != nil {
		if c.ThriftConfig.Host != other.ThriftConfig.Host ||
			c.ThriftConfig.Port != other.ThriftConfig.Port ||
			c.ThriftConfig.ConnectionTimeout != other.ThriftConfig.ConnectionTimeout ||
			c.ThriftConfig.RequestTimeout != other.ThriftConfig.RequestTimeout ||
			c.ThriftConfig.MaxRetries != other.ThriftConfig.MaxRetries ||
			c.ThriftConfig.RetryDelay != other.ThriftConfig.RetryDelay {
			return false
		}
	}

	return true
}
