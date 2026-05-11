package machina

import (
	"context"
	"fmt"
	"sync"
)

// FSM is a type-safe finite state machine parameterized over state type S and
// event type E. Both must be comparable so they can be used as map keys.
//
// FSM is safe for concurrent use. Hooks must not call FSM methods — doing so
// will deadlock since Send holds the write lock for the duration of the transition.
type FSM[S, E comparable] struct {
	mu      sync.RWMutex
	current S
	table   transitionTable[S, E]
	hooks   hookRegistry[S, E]
}

// New creates an FSM starting in state initial with the given options applied.
// It panics if duplicate transitions are registered for the same (from, event) pair.
func New[S, E comparable](initial S, opts ...Option[S, E]) *FSM[S, E] {
	f := &FSM[S, E]{
		current: initial,
		table:   make(transitionTable[S, E]),
		hooks:   newHookRegistry[S, E](),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// State returns the current state.
func (f *FSM[S, E]) State() S {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.current
}

// Can reports whether event e is a valid transition from the current state.
func (f *FSM[S, E]) Can(e E) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.table[f.current][e]
	return ok
}

// States returns all source states that have at least one registered transition.
// The order is not guaranteed.
func (f *FSM[S, E]) States() []S {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]S, 0, len(f.table))
	for s := range f.table {
		out = append(out, s)
	}
	return out
}

// Send fires event e against the current state.
//
// Sequence:
//  1. Validate that (currentState, e) has a registered transition →
//     [ErrInvalidTransition] on failure.
//  2. Run the guard (if any) → [ErrGuardRejected] on false.
//  3. Run OnExit hooks for currentState → abort (state unchanged) on error.
//  4. Advance to the target state.
//  5. Run OnEnter hooks for the new state → return error (state already changed).
//  6. Run OnTransition hooks → return error.
func (f *FSM[S, E]) Send(ctx context.Context, e E) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	tr, ok := f.table[f.current][e]
	if !ok {
		return fmt.Errorf("%w: no transition from %v on event %v", ErrInvalidTransition, f.current, e)
	}

	if tr.guard != nil && !tr.guard(ctx) {
		return fmt.Errorf("%w: %v -[%v]-> %v", ErrGuardRejected, f.current, e, tr.to)
	}

	from := f.current

	for _, h := range f.hooks.onExit[from] {
		if err := h(ctx, e); err != nil {
			return err
		}
	}

	f.current = tr.to

	for _, h := range f.hooks.onEnter[tr.to] {
		if err := h(ctx, e); err != nil {
			return err
		}
	}

	for _, h := range f.hooks.onTransition {
		if err := h(ctx, from, e, tr.to); err != nil {
			return err
		}
	}

	return nil
}
