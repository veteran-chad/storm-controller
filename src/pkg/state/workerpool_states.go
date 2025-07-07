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

// WorkerPoolState represents the state of a Storm worker pool
type WorkerPoolState State

const (
	// WorkerPoolStateUnknown is the initial state
	WorkerPoolStateUnknown WorkerPoolState = "Unknown"

	// WorkerPoolStatePending means the worker pool is pending creation
	WorkerPoolStatePending WorkerPoolState = "Pending"

	// WorkerPoolStateCreating means the worker pool is being created
	WorkerPoolStateCreating WorkerPoolState = "Creating"

	// WorkerPoolStateReady means the worker pool is ready
	WorkerPoolStateReady WorkerPoolState = "Ready"

	// WorkerPoolStateScaling means the worker pool is scaling
	WorkerPoolStateScaling WorkerPoolState = "Scaling"

	// WorkerPoolStateUpdating means the worker pool is updating
	WorkerPoolStateUpdating WorkerPoolState = "Updating"

	// WorkerPoolStateDraining means the worker pool is draining
	WorkerPoolStateDraining WorkerPoolState = "Draining"

	// WorkerPoolStateDeleting means the worker pool is being deleted
	WorkerPoolStateDeleting WorkerPoolState = "Deleting"

	// WorkerPoolStateDeleted means the worker pool is deleted
	WorkerPoolStateDeleted WorkerPoolState = "Deleted"

	// WorkerPoolStateFailed means the worker pool is in a failed state
	WorkerPoolStateFailed WorkerPoolState = "Failed"
)

// WorkerPoolEvent represents events that can occur to a worker pool
type WorkerPoolEvent Event

const (
	// EventWPCreate triggers worker pool creation
	EventWPCreate WorkerPoolEvent = "WPCreate"

	// EventWPCreateComplete means creation completed
	EventWPCreateComplete WorkerPoolEvent = "WPCreateComplete"

	// EventWPCreateFailed means creation failed
	EventWPCreateFailed WorkerPoolEvent = "WPCreateFailed"

	// EventReady means the worker pool is ready
	EventReady WorkerPoolEvent = "Ready"

	// EventScaleUp triggers scale up
	EventScaleUp WorkerPoolEvent = "ScaleUp"

	// EventScaleDown triggers scale down
	EventScaleDown WorkerPoolEvent = "ScaleDown"

	// EventWPScaleComplete means scaling completed
	EventWPScaleComplete WorkerPoolEvent = "ScaleComplete"

	// EventWPScaleFailed means scaling failed
	EventWPScaleFailed WorkerPoolEvent = "ScaleFailed"

	// EventWPUpdateConfig triggers configuration update
	EventWPUpdateConfig WorkerPoolEvent = "UpdateConfig"

	// EventWPUpdateComplete means update completed
	EventWPUpdateComplete WorkerPoolEvent = "UpdateComplete"

	// EventWPUpdateFailed means update failed
	EventWPUpdateFailed WorkerPoolEvent = "UpdateFailed"

	// EventWPDrain triggers draining
	EventWPDrain WorkerPoolEvent = "Drain"

	// EventWPDrainComplete means draining completed
	EventWPDrainComplete WorkerPoolEvent = "DrainComplete"

	// EventWPDelete triggers deletion
	EventWPDelete WorkerPoolEvent = "Delete"

	// EventWPDeleteComplete means deletion completed
	EventWPDeleteComplete WorkerPoolEvent = "DeleteComplete"

	// EventWPRecover triggers recovery
	EventWPRecover WorkerPoolEvent = "Recover"
)

// NewWorkerPoolStateMachine creates a state machine for Storm worker pool lifecycle
func NewWorkerPoolStateMachine() *StateMachine {
	sm := NewStateMachine(State(WorkerPoolStateUnknown))

	// Define state transitions
	// From Unknown
	sm.AddTransition(State(WorkerPoolStateUnknown), Event(EventWPCreate), State(WorkerPoolStateCreating))

	// From Pending
	sm.AddTransition(State(WorkerPoolStatePending), Event(EventWPCreate), State(WorkerPoolStateCreating))

	// From Creating
	sm.AddTransition(State(WorkerPoolStateCreating), Event(EventWPCreateComplete), State(WorkerPoolStateReady))
	sm.AddTransition(State(WorkerPoolStateCreating), Event(EventWPCreateFailed), State(WorkerPoolStateFailed))

	// From Ready
	sm.AddTransition(State(WorkerPoolStateReady), Event(EventScaleUp), State(WorkerPoolStateScaling))
	sm.AddTransition(State(WorkerPoolStateReady), Event(EventScaleDown), State(WorkerPoolStateScaling))
	sm.AddTransition(State(WorkerPoolStateReady), Event(EventWPUpdateConfig), State(WorkerPoolStateUpdating))
	sm.AddTransition(State(WorkerPoolStateReady), Event(EventWPDrain), State(WorkerPoolStateDraining))
	sm.AddTransition(State(WorkerPoolStateReady), Event(EventWPDelete), State(WorkerPoolStateDeleting))

	// From Scaling
	sm.AddTransition(State(WorkerPoolStateScaling), Event(EventWPScaleComplete), State(WorkerPoolStateReady))
	sm.AddTransition(State(WorkerPoolStateScaling), Event(EventWPScaleFailed), State(WorkerPoolStateFailed))

	// From Updating
	sm.AddTransition(State(WorkerPoolStateUpdating), Event(EventWPUpdateComplete), State(WorkerPoolStateReady))
	sm.AddTransition(State(WorkerPoolStateUpdating), Event(EventWPUpdateFailed), State(WorkerPoolStateFailed))

	// From Draining
	sm.AddTransition(State(WorkerPoolStateDraining), Event(EventWPDrainComplete), State(WorkerPoolStateReady))
	sm.AddTransition(State(WorkerPoolStateDraining), Event(EventWPDelete), State(WorkerPoolStateDeleting))

	// From Deleting
	sm.AddTransition(State(WorkerPoolStateDeleting), Event(EventWPDeleteComplete), State(WorkerPoolStateDeleted))

	// From Failed
	sm.AddTransition(State(WorkerPoolStateFailed), Event(EventWPRecover), State(WorkerPoolStatePending))
	sm.AddTransition(State(WorkerPoolStateFailed), Event(EventWPDelete), State(WorkerPoolStateDeleting))

	return sm
}
