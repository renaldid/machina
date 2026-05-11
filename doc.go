// Package machina provides a type-safe finite state machine for Go,
// parameterized over state and event types using generics.
//
// Unlike string-based FSM libraries (such as looplab/fsm), machina uses any
// comparable type for states and events — typically typed integers or custom
// types — catching invalid state and event usage at compile time.
//
// # Basic usage
//
//	type State int
//	const ( Pending State = iota; Active; Closed )
//
//	type Event int
//	const ( Activate Event = iota; Close )
//
//	m := machina.New[State, Event](Pending,
//	    machina.T(Pending, Activate, Active),
//	    machina.T(Active,  Close,    Closed),
//	    machina.T(Pending, Close,    Closed),
//	)
//
//	if err := m.Send(ctx, Activate); err != nil { ... }
//	fmt.Println(m.State()) // Active
//
// # Guards
//
// Use [TG] to register a transition that only proceeds when a guard returns true:
//
//	machina.TG(Pending, Close, Closed, func(ctx context.Context) bool {
//	    return isAdmin(ctx)
//	})
//
// # Hooks
//
// [OnEnter], [OnExit], and [OnTransition] register callbacks:
//
//	machina.OnEnter(Active, func(ctx context.Context, e Event) error {
//	    return sendWelcomeEmail(ctx)
//	})
//	machina.OnTransition(func(ctx context.Context, from State, e Event, to State) error {
//	    return auditLog(ctx, from, e, to)
//	})
//
// Hooks registered for the same state are called in registration order.
// If an [OnExit] hook returns an error the transition is aborted and the state
// remains unchanged. If an [OnEnter] hook returns an error the state has
// already changed.
//
// # Visualization
//
// [FSM.DOT] returns a Graphviz DOT string that can be piped to dot(1):
//
//	fmt.Println(m.DOT())
//	// digraph machina {
//	//     rankdir=LR;
//	//     "0" -> "1" [label="0"];
//	//     ...
//	// }
//
// # Concurrency
//
// FSM is safe for concurrent use. [FSM.Send] holds the write lock for the
// duration of the full transition (guard + hooks + state change). Hooks must
// not call FSM methods to avoid deadlocking.
package machina
