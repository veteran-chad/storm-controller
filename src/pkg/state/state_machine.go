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
	"fmt"
	"time"
)

// State represents a state in the state machine
type State string

// Event represents an event that triggers a state transition
type Event string

// TransitionFunc is called during a state transition
type TransitionFunc func(ctx context.Context, from State, to State, event Event) error

// StateHandler handles actions for a specific state
type StateHandler func(ctx context.Context) (Event, error)

// StateMachine represents a finite state machine
type StateMachine struct {
	currentState State
	transitions  map[State]map[Event]State
	handlers     map[State]StateHandler
	onTransition TransitionFunc
	history      []TransitionRecord
}

// TransitionRecord records a state transition
type TransitionRecord struct {
	From      State
	To        State
	Event     Event
	Timestamp time.Time
	Error     error
}

// NewStateMachine creates a new state machine
func NewStateMachine(initialState State) *StateMachine {
	return &StateMachine{
		currentState: initialState,
		transitions:  make(map[State]map[Event]State),
		handlers:     make(map[State]StateHandler),
		history:      make([]TransitionRecord, 0),
	}
}

// AddTransition adds a state transition rule
func (sm *StateMachine) AddTransition(from State, event Event, to State) {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[Event]State)
	}
	sm.transitions[from][event] = to
}

// SetHandler sets the handler for a state
func (sm *StateMachine) SetHandler(state State, handler StateHandler) {
	sm.handlers[state] = handler
}

// SetTransitionFunc sets the function to be called on transitions
func (sm *StateMachine) SetTransitionFunc(fn TransitionFunc) {
	sm.onTransition = fn
}

// CurrentState returns the current state
func (sm *StateMachine) CurrentState() State {
	return sm.currentState
}

// History returns the transition history
func (sm *StateMachine) History() []TransitionRecord {
	return sm.history
}

// ProcessEvent processes an event and potentially transitions to a new state
func (sm *StateMachine) ProcessEvent(ctx context.Context, event Event) error {
	transitions, ok := sm.transitions[sm.currentState]
	if !ok {
		return fmt.Errorf("no transitions defined for state %s", sm.currentState)
	}

	nextState, ok := transitions[event]
	if !ok {
		return fmt.Errorf("no transition for event %s in state %s", event, sm.currentState)
	}

	// Record the transition
	record := TransitionRecord{
		From:      sm.currentState,
		To:        nextState,
		Event:     event,
		Timestamp: time.Now(),
	}

	// Call transition function if set
	if sm.onTransition != nil {
		if err := sm.onTransition(ctx, sm.currentState, nextState, event); err != nil {
			record.Error = err
			sm.history = append(sm.history, record)
			return fmt.Errorf("transition function failed: %w", err)
		}
	}

	// Update state
	sm.currentState = nextState
	sm.history = append(sm.history, record)

	return nil
}

// Run executes the state machine until it reaches a terminal state or error
func (sm *StateMachine) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		handler, ok := sm.handlers[sm.currentState]
		if !ok {
			// No handler means this is a terminal state
			return nil
		}

		event, err := handler(ctx)
		if err != nil {
			return fmt.Errorf("handler for state %s failed: %w", sm.currentState, err)
		}

		if event == "" {
			// No event means stay in current state
			continue
		}

		if err := sm.ProcessEvent(ctx, event); err != nil {
			return err
		}
	}
}

// Validate checks if the state machine is properly configured
func (sm *StateMachine) Validate() error {
	// Check for unreachable states
	reachable := make(map[State]bool)
	reachable[sm.currentState] = true

	// BFS to find all reachable states
	queue := []State{sm.currentState}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if transitions, ok := sm.transitions[current]; ok {
			for _, nextState := range transitions {
				if !reachable[nextState] {
					reachable[nextState] = true
					queue = append(queue, nextState)
				}
			}
		}
	}

	// Check if all states with handlers are reachable
	for state := range sm.handlers {
		if !reachable[state] {
			return fmt.Errorf("state %s has handler but is unreachable", state)
		}
	}

	// Check for states without transitions (should be terminal states)
	for state := range reachable {
		if _, hasTransitions := sm.transitions[state]; !hasTransitions {
			if _, hasHandler := sm.handlers[state]; hasHandler {
				// Non-terminal state without transitions
				return fmt.Errorf("state %s has handler but no transitions", state)
			}
		}
	}

	return nil
}
