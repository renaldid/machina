package machina

import (
	"context"
	"fmt"
)

// HookFn is a callback invoked on state entry or exit.
// The event that triggered the transition is passed as the second argument.
type HookFn[E any] func(ctx context.Context, event E) error

// TransitionHookFn is a callback invoked after every successful transition.
type TransitionHookFn[S, E any] func(ctx context.Context, from S, event E, to S) error

// GuardFn is a predicate that must return true for a transition to be allowed.
type GuardFn func(ctx context.Context) bool

// Option configures an [FSM] during construction.
type Option[S, E comparable] func(*FSM[S, E])

// transition holds the target state and optional guard for one (from, event) pair.
type transition[S, E comparable] struct {
	to    S
	guard GuardFn
}

// transitionTable maps from-state → event → transition.
type transitionTable[S, E comparable] map[S]map[E]transition[S, E]

// hookRegistry holds all registered hooks.
type hookRegistry[S, E comparable] struct {
	onEnter      map[S][]HookFn[E]
	onExit       map[S][]HookFn[E]
	onTransition []TransitionHookFn[S, E]
}

func newHookRegistry[S, E comparable]() hookRegistry[S, E] {
	return hookRegistry[S, E]{
		onEnter: make(map[S][]HookFn[E]),
		onExit:  make(map[S][]HookFn[E]),
	}
}

// T registers a transition: from state `from`, on event `event`, move to state `to`.
// It panics if a transition for (from, event) is already registered.
func T[S, E comparable](from S, event E, to S) Option[S, E] {
	return func(f *FSM[S, E]) {
		registerTransition(f, from, event, transition[S, E]{to: to})
	}
}

// TG registers a guarded transition. The transition proceeds only if guard returns true.
// It panics if a transition for (from, event) is already registered.
func TG[S, E comparable](from S, event E, to S, guard GuardFn) Option[S, E] {
	return func(f *FSM[S, E]) {
		registerTransition(f, from, event, transition[S, E]{to: to, guard: guard})
	}
}

func registerTransition[S, E comparable](f *FSM[S, E], from S, event E, tr transition[S, E]) {
	if f.table[from] == nil {
		f.table[from] = make(map[E]transition[S, E])
	}
	if _, exists := f.table[from][event]; exists {
		panic(fmt.Sprintf("machina: duplicate transition from %v on event %v", from, event))
	}
	f.table[from][event] = tr
}

// OnEnter registers fn to be called when the FSM enters state s.
// Multiple hooks for the same state are called in registration order.
func OnEnter[S, E comparable](state S, fn HookFn[E]) Option[S, E] {
	return func(f *FSM[S, E]) {
		f.hooks.onEnter[state] = append(f.hooks.onEnter[state], fn)
	}
}

// OnExit registers fn to be called when the FSM exits state s.
// If fn returns an error, the transition is aborted and the state remains unchanged.
// Multiple hooks for the same state are called in registration order.
func OnExit[S, E comparable](state S, fn HookFn[E]) Option[S, E] {
	return func(f *FSM[S, E]) {
		f.hooks.onExit[state] = append(f.hooks.onExit[state], fn)
	}
}

// OnTransition registers fn to be called after every successful transition.
// Multiple hooks are called in registration order.
func OnTransition[S, E comparable](fn TransitionHookFn[S, E]) Option[S, E] {
	return func(f *FSM[S, E]) {
		f.hooks.onTransition = append(f.hooks.onTransition, fn)
	}
}
