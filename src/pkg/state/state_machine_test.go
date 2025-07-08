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

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStateMachine_BasicTransitions(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "middle")
	sm.AddTransition("middle", "go", "end")

	// Test initial state
	if sm.CurrentState() != "start" {
		t.Errorf("Expected initial state 'start', got %s", sm.CurrentState())
	}

	// Test valid transition
	err := sm.ProcessEvent(context.Background(), "go")
	if err != nil {
		t.Errorf("Expected successful transition, got error: %v", err)
	}
	if sm.CurrentState() != "middle" {
		t.Errorf("Expected state 'middle', got %s", sm.CurrentState())
	}

	// Test another valid transition
	err = sm.ProcessEvent(context.Background(), "go")
	if err != nil {
		t.Errorf("Expected successful transition, got error: %v", err)
	}
	if sm.CurrentState() != "end" {
		t.Errorf("Expected state 'end', got %s", sm.CurrentState())
	}

	// Test invalid transition
	err = sm.ProcessEvent(context.Background(), "go")
	if err == nil {
		t.Error("Expected error for invalid transition, got nil")
	}
}

func TestStateMachine_TransitionFunc(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "end")

	var called bool
	var fromState, toState State
	var event Event

	sm.SetTransitionFunc(func(ctx context.Context, from, to State, evt Event) error {
		called = true
		fromState = from
		toState = to
		event = evt
		return nil
	})

	err := sm.ProcessEvent(context.Background(), "go")
	if err != nil {
		t.Errorf("Expected successful transition, got error: %v", err)
	}

	if !called {
		t.Error("Expected transition function to be called")
	}
	if fromState != "start" || toState != "end" || event != "go" {
		t.Errorf("Transition function called with wrong parameters: from=%s, to=%s, event=%s",
			fromState, toState, event)
	}
}

func TestStateMachine_TransitionFuncError(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "end")

	sm.SetTransitionFunc(func(ctx context.Context, from, to State, evt Event) error {
		return errors.New("transition error")
	})

	err := sm.ProcessEvent(context.Background(), "go")
	if err == nil {
		t.Error("Expected error from transition function")
	}

	// State should not have changed
	if sm.CurrentState() != "start" {
		t.Errorf("Expected state to remain 'start', got %s", sm.CurrentState())
	}
}

func TestStateMachine_History(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "middle")
	sm.AddTransition("middle", "back", "start")

	// Make some transitions
	sm.ProcessEvent(context.Background(), "go")
	sm.ProcessEvent(context.Background(), "back")

	history := sm.History()
	if len(history) != 2 {
		t.Errorf("Expected 2 history records, got %d", len(history))
	}

	// Check first transition
	if history[0].From != "start" || history[0].To != "middle" || history[0].Event != "go" {
		t.Errorf("First history record incorrect: %+v", history[0])
	}

	// Check second transition
	if history[1].From != "middle" || history[1].To != "start" || history[1].Event != "back" {
		t.Errorf("Second history record incorrect: %+v", history[1])
	}
}

func TestStateMachine_Run(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "middle")
	sm.AddTransition("middle", "go", "end")

	callCount := 0
	sm.SetHandler("start", func(ctx context.Context) (Event, error) {
		callCount++
		return "go", nil
	})
	sm.SetHandler("middle", func(ctx context.Context) (Event, error) {
		callCount++
		return "go", nil
	})
	// No handler for "end" - it's a terminal state

	err := sm.Run(context.Background())
	if err != nil {
		t.Errorf("Expected successful run, got error: %v", err)
	}

	if sm.CurrentState() != "end" {
		t.Errorf("Expected final state 'end', got %s", sm.CurrentState())
	}

	if callCount != 2 {
		t.Errorf("Expected 2 handler calls, got %d", callCount)
	}
}

func TestStateMachine_RunWithContext(t *testing.T) {
	sm := NewStateMachine("start")
	sm.AddTransition("start", "go", "middle")

	// Handler that blocks
	sm.SetHandler("start", func(ctx context.Context) (Event, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := sm.Run(ctx)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	// The error could be wrapped, so check if it contains the deadline exceeded error
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context deadline exceeded error, got %v", err)
	}
}

func TestStateMachine_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *StateMachine
		expectError bool
	}{
		{
			name: "valid state machine",
			setup: func() *StateMachine {
				sm := NewStateMachine("start")
				sm.AddTransition("start", "go", "end")
				sm.SetHandler("start", func(ctx context.Context) (Event, error) {
					return "go", nil
				})
				return sm
			},
			expectError: false,
		},
		{
			name: "unreachable state",
			setup: func() *StateMachine {
				sm := NewStateMachine("start")
				sm.AddTransition("start", "go", "middle")
				// "end" state has handler but is unreachable
				sm.SetHandler("end", func(ctx context.Context) (Event, error) {
					return "", nil
				})
				return sm
			},
			expectError: true,
		},
		{
			name: "non-terminal state without transitions",
			setup: func() *StateMachine {
				sm := NewStateMachine("start")
				// "start" has handler but no transitions
				sm.SetHandler("start", func(ctx context.Context) (Event, error) {
					return "go", nil
				})
				return sm
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := tt.setup()
			err := sm.Validate()
			if (err != nil) != tt.expectError {
				t.Errorf("Validate() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestTopologyStateMachine(t *testing.T) {
	sm := NewTopologyStateMachine()

	// Test a typical topology lifecycle
	transitions := []struct {
		event         Event
		expectedState State
	}{
		{Event(EventValidate), State(TopologyStateValidating)},
		{Event(EventValidationSuccess), State(TopologyStateDownloading)},
		{Event(EventDownloadComplete), State(TopologyStateSubmitting)},
		{Event(EventSubmitSuccess), State(TopologyStateRunning)},
		{Event(EventSuspend), State(TopologyStateSuspended)},
		{Event(EventResume), State(TopologyStateRunning)},
		{Event(EventKill), State(TopologyStateKilling)},
		{Event(EventKillComplete), State(TopologyStateKilled)},
	}

	for _, tt := range transitions {
		err := sm.ProcessEvent(context.Background(), tt.event)
		if err != nil {
			t.Errorf("Failed to process event %s: %v", tt.event, err)
		}
		if sm.CurrentState() != tt.expectedState {
			t.Errorf("After event %s, expected state %s, got %s",
				tt.event, tt.expectedState, sm.CurrentState())
		}
	}
}

func TestClusterStateMachine(t *testing.T) {
	sm := NewClusterStateMachine()

	// Test a typical cluster lifecycle
	transitions := []struct {
		event         Event
		expectedState State
	}{
		{Event(EventCreate), State(ClusterStatePending)},
		{Event(EventCreate), State(ClusterStateCreating)},
		{Event(EventCreateComplete), State(ClusterStateRunning)},
		{Event(EventUnhealthy), State(ClusterStateFailed)},
		{Event(EventRecover), State(ClusterStatePending)},
		{Event(EventCreate), State(ClusterStateCreating)},
		{Event(EventCreateComplete), State(ClusterStateRunning)},
		{Event(EventUpdate), State(ClusterStateUpdating)},
		{Event(EventUpdateComplete), State(ClusterStateRunning)},
		{Event(EventTerminate), State(ClusterStateTerminating)},
	}

	for _, tt := range transitions {
		err := sm.ProcessEvent(context.Background(), tt.event)
		if err != nil {
			t.Errorf("Failed to process event %s: %v", tt.event, err)
		}
		if sm.CurrentState() != tt.expectedState {
			t.Errorf("After event %s, expected state %s, got %s",
				tt.event, tt.expectedState, sm.CurrentState())
		}
	}
}

func TestWorkerPoolStateMachine(t *testing.T) {
	sm := NewWorkerPoolStateMachine()

	// Test a typical worker pool lifecycle
	transitions := []struct {
		event         Event
		expectedState State
	}{
		{Event(EventWPCreate), State(WorkerPoolStateCreating)},
		{Event(EventWPCreateComplete), State(WorkerPoolStateReady)},
		{Event(EventScaleUp), State(WorkerPoolStateScaling)},
		{Event(EventWPScaleComplete), State(WorkerPoolStateReady)},
		{Event(EventWPUpdateConfig), State(WorkerPoolStateUpdating)},
		{Event(EventWPUpdateComplete), State(WorkerPoolStateReady)},
		{Event(EventWPDrain), State(WorkerPoolStateDraining)},
		{Event(EventWPDrainComplete), State(WorkerPoolStateReady)},
		{Event(EventWPDelete), State(WorkerPoolStateDeleting)},
		{Event(EventWPDeleteComplete), State(WorkerPoolStateDeleted)},
	}

	for _, tt := range transitions {
		err := sm.ProcessEvent(context.Background(), tt.event)
		if err != nil {
			t.Errorf("Failed to process event %s: %v", tt.event, err)
		}
		if sm.CurrentState() != tt.expectedState {
			t.Errorf("After event %s, expected state %s, got %s",
				tt.event, tt.expectedState, sm.CurrentState())
		}
	}
}
