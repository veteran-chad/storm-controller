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

package config

// ControllerConfig holds configuration for the Storm controller
type ControllerConfig struct {
	// UseStateMachine enables state machine-based controllers
	UseStateMachine bool

	// EnableEnhancedControllers enables enhanced controller features
	EnableEnhancedControllers bool

	// ThriftEnabled enables Thrift client for Storm communication
	ThriftEnabled bool

	// MetricsEnabled enables Prometheus metrics
	MetricsEnabled bool
}

// NewDefaultConfig returns a default controller configuration
func NewDefaultConfig() *ControllerConfig {
	return &ControllerConfig{
		UseStateMachine:           false, // Default to simple controllers
		EnableEnhancedControllers: true,  // Use enhanced controllers by default
		ThriftEnabled:             true,  // Use Thrift by default
		MetricsEnabled:            true,  // Enable metrics by default
	}
}
