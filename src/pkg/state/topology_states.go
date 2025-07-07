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

// TopologyState represents the state of a Storm topology
type TopologyState State

const (
	// TopologyStateUnknown is the initial state
	TopologyStateUnknown TopologyState = "Unknown"

	// TopologyStatePending means the topology is pending submission
	TopologyStatePending TopologyState = "Pending"

	// TopologyStateValidating means the topology is being validated
	TopologyStateValidating TopologyState = "Validating"

	// TopologyStateDownloading means the JAR is being downloaded
	TopologyStateDownloading TopologyState = "Downloading"

	// TopologyStateSubmitting means the topology is being submitted
	TopologyStateSubmitting TopologyState = "Submitting"

	// TopologyStateRunning means the topology is running
	TopologyStateRunning TopologyState = "Running"

	// TopologyStateSuspended means the topology is suspended
	TopologyStateSuspended TopologyState = "Suspended"

	// TopologyStateUpdating means the topology is being updated
	TopologyStateUpdating TopologyState = "Updating"

	// TopologyStateKilling means the topology is being killed
	TopologyStateKilling TopologyState = "Killing"

	// TopologyStateKilled means the topology has been killed
	TopologyStateKilled TopologyState = "Killed"

	// TopologyStateFailed means the topology is in a failed state
	TopologyStateFailed TopologyState = "Failed"
)

// TopologyEvent represents events that can occur to a topology
type TopologyEvent Event

const (
	// EventValidate triggers validation
	EventValidate TopologyEvent = "Validate"

	// EventValidationSuccess means validation succeeded
	EventValidationSuccess TopologyEvent = "ValidationSuccess"

	// EventValidationFailed means validation failed
	EventValidationFailed TopologyEvent = "ValidationFailed"

	// EventDownloadJAR triggers JAR download
	EventDownloadJAR TopologyEvent = "DownloadJAR"

	// EventDownloadComplete means JAR download completed
	EventDownloadComplete TopologyEvent = "DownloadComplete"

	// EventDownloadFailed means JAR download failed
	EventDownloadFailed TopologyEvent = "DownloadFailed"

	// EventSubmit triggers topology submission
	EventSubmit TopologyEvent = "Submit"

	// EventSubmitSuccess means submission succeeded
	EventSubmitSuccess TopologyEvent = "SubmitSuccess"

	// EventSubmitFailed means submission failed
	EventSubmitFailed TopologyEvent = "SubmitFailed"

	// EventSuspend triggers topology suspension
	EventSuspend TopologyEvent = "Suspend"

	// EventResume triggers topology resumption
	EventResume TopologyEvent = "Resume"

	// EventTopologyUpdate triggers topology update
	EventTopologyUpdate TopologyEvent = "TopologyUpdate"

	// EventTopologyUpdateComplete means update completed
	EventTopologyUpdateComplete TopologyEvent = "TopologyUpdateComplete"

	// EventKill triggers topology termination
	EventKill TopologyEvent = "Kill"

	// EventKillComplete means topology was killed
	EventKillComplete TopologyEvent = "KillComplete"

	// EventError indicates an error occurred
	EventError TopologyEvent = "Error"

	// EventRetry triggers a retry
	EventRetry TopologyEvent = "Retry"
)

// NewTopologyStateMachine creates a state machine for Storm topology lifecycle
func NewTopologyStateMachine() *StateMachine {
	sm := NewStateMachine(State(TopologyStateUnknown))

	// Define state transitions
	// From Unknown
	sm.AddTransition(State(TopologyStateUnknown), Event(EventValidate), State(TopologyStateValidating))

	// From Pending
	sm.AddTransition(State(TopologyStatePending), Event(EventValidate), State(TopologyStateValidating))
	sm.AddTransition(State(TopologyStatePending), Event(EventKill), State(TopologyStateKilled))

	// From Validating
	sm.AddTransition(State(TopologyStateValidating), Event(EventValidationSuccess), State(TopologyStateDownloading))
	sm.AddTransition(State(TopologyStateValidating), Event(EventValidationFailed), State(TopologyStateFailed))

	// From Downloading
	sm.AddTransition(State(TopologyStateDownloading), Event(EventDownloadComplete), State(TopologyStateSubmitting))
	sm.AddTransition(State(TopologyStateDownloading), Event(EventDownloadFailed), State(TopologyStateFailed))

	// From Submitting
	sm.AddTransition(State(TopologyStateSubmitting), Event(EventSubmitSuccess), State(TopologyStateRunning))
	sm.AddTransition(State(TopologyStateSubmitting), Event(EventSubmitFailed), State(TopologyStateFailed))

	// From Running
	sm.AddTransition(State(TopologyStateRunning), Event(EventSuspend), State(TopologyStateSuspended))
	sm.AddTransition(State(TopologyStateRunning), Event(EventTopologyUpdate), State(TopologyStateUpdating))
	sm.AddTransition(State(TopologyStateRunning), Event(EventKill), State(TopologyStateKilling))
	sm.AddTransition(State(TopologyStateRunning), Event(EventError), State(TopologyStateFailed))

	// From Suspended
	sm.AddTransition(State(TopologyStateSuspended), Event(EventResume), State(TopologyStateRunning))
	sm.AddTransition(State(TopologyStateSuspended), Event(EventKill), State(TopologyStateKilling))

	// From Updating
	sm.AddTransition(State(TopologyStateUpdating), Event(EventTopologyUpdateComplete), State(TopologyStateRunning))
	sm.AddTransition(State(TopologyStateUpdating), Event(EventError), State(TopologyStateFailed))

	// From Killing
	sm.AddTransition(State(TopologyStateKilling), Event(EventKillComplete), State(TopologyStateKilled))
	sm.AddTransition(State(TopologyStateKilling), Event(EventError), State(TopologyStateFailed))

	// From Failed
	sm.AddTransition(State(TopologyStateFailed), Event(EventRetry), State(TopologyStatePending))
	sm.AddTransition(State(TopologyStateFailed), Event(EventKill), State(TopologyStateKilled))

	return sm
}
