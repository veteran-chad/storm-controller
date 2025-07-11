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

package state

// ClusterState represents the state of a Storm cluster
type ClusterState State

const (
	// ClusterStateUnknown is the initial state
	ClusterStateUnknown ClusterState = "Unknown"

	// ClusterStatePending means the cluster is pending creation
	ClusterStatePending ClusterState = "Pending"

	// ClusterStateCreating means the cluster is being created/provisioned
	ClusterStateCreating ClusterState = "Creating"

	// ClusterStateRunning means the cluster is running
	ClusterStateRunning ClusterState = "Running"

	// ClusterStateFailed means the cluster is in a failed state
	ClusterStateFailed ClusterState = "Failed"

	// ClusterStateUpdating means the cluster is being updated
	ClusterStateUpdating ClusterState = "Updating"

	// ClusterStateTerminating means the cluster is terminating
	ClusterStateTerminating ClusterState = "Terminating"
)

// ClusterEvent represents events that can occur to a cluster
type ClusterEvent Event

const (
	// EventCreate triggers cluster creation
	EventCreate ClusterEvent = "Create"

	// EventCreateComplete means creation completed
	EventCreateComplete ClusterEvent = "CreateComplete"

	// EventCreateFailed means creation failed
	EventCreateFailed ClusterEvent = "CreateFailed"

	// EventHealthy means the cluster became healthy
	EventHealthy ClusterEvent = "Healthy"

	// EventUnhealthy means the cluster became unhealthy
	EventUnhealthy ClusterEvent = "Unhealthy"

	// EventUpdate triggers update/upgrade
	EventUpdate ClusterEvent = "Update"

	// EventUpdateComplete means update completed
	EventUpdateComplete ClusterEvent = "UpdateComplete"

	// EventUpdateFailed means update failed
	EventUpdateFailed ClusterEvent = "UpdateFailed"

	// EventTerminate triggers termination
	EventTerminate ClusterEvent = "Terminate"

	// EventTerminateComplete means termination completed
	EventTerminateComplete ClusterEvent = "TerminateComplete"

	// EventRecover triggers recovery
	EventRecover ClusterEvent = "Recover"
)

// NewClusterStateMachine creates a state machine for Storm cluster lifecycle
func NewClusterStateMachine() *StateMachine {
	sm := NewStateMachine(State(ClusterStateUnknown))

	// Define state transitions
	// From Unknown
	sm.AddTransition(State(ClusterStateUnknown), Event(EventCreate), State(ClusterStatePending))

	// From Pending
	sm.AddTransition(State(ClusterStatePending), Event(EventCreate), State(ClusterStateCreating))

	// From Creating
	sm.AddTransition(State(ClusterStateCreating), Event(EventCreateComplete), State(ClusterStateRunning))
	sm.AddTransition(State(ClusterStateCreating), Event(EventCreateFailed), State(ClusterStateFailed))

	// From Running
	sm.AddTransition(State(ClusterStateRunning), Event(EventUnhealthy), State(ClusterStateFailed))
	sm.AddTransition(State(ClusterStateRunning), Event(EventUpdate), State(ClusterStateUpdating))
	sm.AddTransition(State(ClusterStateRunning), Event(EventTerminate), State(ClusterStateTerminating))

	// From Updating
	sm.AddTransition(State(ClusterStateUpdating), Event(EventUpdateComplete), State(ClusterStateRunning))
	sm.AddTransition(State(ClusterStateUpdating), Event(EventUpdateFailed), State(ClusterStateFailed))

	// From Failed
	sm.AddTransition(State(ClusterStateFailed), Event(EventRecover), State(ClusterStatePending))
	sm.AddTransition(State(ClusterStateFailed), Event(EventTerminate), State(ClusterStateTerminating))

	return sm
}
