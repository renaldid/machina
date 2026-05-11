package machina_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/renaldid/machina"
)

// Test state/event types
type S int

const (
	Idle S = iota
	Running
	Paused
	Done
)

type E int

const (
	Start E = iota
	Pause
	Resume
	Stop
	Cancel
)

var ctx = context.Background()

func newBasic() *machina.FSM[S, E] {
	return machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.T(Running, Pause, Paused),
		machina.T(Running, Stop, Done),
		machina.T(Paused, Resume, Running),
		machina.T(Paused, Stop, Done),
		machina.T(Idle, Cancel, Done),
	)
}

// --- State and Can ---

func TestState_Initial(t *testing.T) {
	m := newBasic()
	if got := m.State(); got != Idle {
		t.Fatalf("want Idle, got %v", got)
	}
}

func TestCan_Valid(t *testing.T) {
	m := newBasic()
	if !m.Can(Start) {
		t.Fatal("expected Can(Start) = true from Idle")
	}
}

func TestCan_Invalid(t *testing.T) {
	m := newBasic()
	if m.Can(Pause) {
		t.Fatal("expected Can(Pause) = false from Idle")
	}
}

// --- Send: basic ---

func TestSend_ValidTransition(t *testing.T) {
	m := newBasic()
	if err := m.Send(ctx, Start); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.State(); got != Running {
		t.Fatalf("want Running, got %v", got)
	}
}

func TestSend_ErrInvalidTransition_NoEventsFromState(t *testing.T) {
	m := newBasic()
	// Done has no outgoing transitions
	m.Send(ctx, Start)
	m.Send(ctx, Stop)
	err := m.Send(ctx, Start)
	if !errors.Is(err, machina.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestSend_ErrInvalidTransition_WrongEvent(t *testing.T) {
	m := newBasic()
	// Idle has transitions but not for Pause
	err := m.Send(ctx, Pause)
	if !errors.Is(err, machina.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

// --- Guard ---

func TestSend_GuardAllowed(t *testing.T) {
	allow := true
	m := machina.New[S, E](Idle,
		machina.TG(Idle, Start, Running, func(ctx context.Context) bool { return allow }),
	)
	if err := m.Send(ctx, Start); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.State() != Running {
		t.Fatalf("want Running")
	}
}

func TestSend_GuardRejected(t *testing.T) {
	m := machina.New[S, E](Idle,
		machina.TG(Idle, Start, Running, func(ctx context.Context) bool { return false }),
	)
	err := m.Send(ctx, Start)
	if !errors.Is(err, machina.ErrGuardRejected) {
		t.Fatalf("want ErrGuardRejected, got %v", err)
	}
	if m.State() != Idle {
		t.Fatal("state must not change when guard rejects")
	}
}

// --- Hooks ---

func TestOnExit_CalledBeforeTransition(t *testing.T) {
	var exitState S = -1
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnExit[S, E](Idle, func(ctx context.Context, e E) error {
			exitState = Idle // record that we were still in Idle
			return nil
		}),
	)
	m.Send(ctx, Start)
	if exitState != Idle {
		t.Fatal("OnExit should be called with old state")
	}
}

func TestOnExit_ErrorAbortsTransition(t *testing.T) {
	boom := errors.New("exit failed")
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnExit[S, E](Idle, func(ctx context.Context, e E) error { return boom }),
	)
	err := m.Send(ctx, Start)
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if m.State() != Idle {
		t.Fatal("state must not change when OnExit returns error")
	}
}

func TestOnEnter_CalledAfterTransition(t *testing.T) {
	var entered bool
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnEnter[S, E](Running, func(ctx context.Context, e E) error {
			entered = true
			return nil
		}),
	)
	m.Send(ctx, Start)
	if !entered {
		t.Fatal("OnEnter should be called")
	}
}

func TestOnEnter_ErrorReturnedStateAlreadyChanged(t *testing.T) {
	boom := errors.New("enter failed")
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnEnter[S, E](Running, func(ctx context.Context, e E) error { return boom }),
	)
	err := m.Send(ctx, Start)
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	// State has already changed (OnEnter runs after transition)
	if m.State() != Running {
		t.Fatal("state should have changed even when OnEnter returns error")
	}
}

func TestOnTransition_Called(t *testing.T) {
	var recorded bool
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnTransition[S, E](func(ctx context.Context, from S, e E, to S) error {
			if from == Idle && e == Start && to == Running {
				recorded = true
			}
			return nil
		}),
	)
	m.Send(ctx, Start)
	if !recorded {
		t.Fatal("OnTransition hook not called")
	}
}

func TestOnTransition_ErrorReturned(t *testing.T) {
	boom := errors.New("transition hook failed")
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnTransition[S, E](func(ctx context.Context, from S, e E, to S) error { return boom }),
	)
	err := m.Send(ctx, Start)
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}

func TestMultipleHooks_CalledInOrder(t *testing.T) {
	var calls []int
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.OnExit[S, E](Idle, func(ctx context.Context, e E) error { calls = append(calls, 1); return nil }),
		machina.OnExit[S, E](Idle, func(ctx context.Context, e E) error { calls = append(calls, 2); return nil }),
		machina.OnEnter[S, E](Running, func(ctx context.Context, e E) error { calls = append(calls, 3); return nil }),
		machina.OnEnter[S, E](Running, func(ctx context.Context, e E) error { calls = append(calls, 4); return nil }),
	)
	m.Send(ctx, Start)
	want := []int{1, 2, 3, 4}
	for i, v := range want {
		if calls[i] != v {
			t.Fatalf("hook order: want %v, got %v", want, calls)
		}
	}
}

// --- States ---

func TestStates(t *testing.T) {
	m := newBasic()
	states := m.States()
	// Should contain Idle, Running, Paused (Done has no outgoing transitions)
	set := make(map[S]bool)
	for _, s := range states {
		set[s] = true
	}
	for _, want := range []S{Idle, Running, Paused} {
		if !set[want] {
			t.Errorf("States() missing %v", want)
		}
	}
	if set[Done] {
		t.Error("States() should not include terminal state Done")
	}
}

// --- DOT ---

func TestDOT_ContainsTransitions(t *testing.T) {
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
	)
	dot := m.DOT()
	if !strings.Contains(dot, "digraph machina") {
		t.Error("DOT missing header")
	}
	if !strings.Contains(dot, "rankdir=LR") {
		t.Error("DOT missing rankdir")
	}
	// Should contain an edge
	if !strings.Contains(dot, "->") {
		t.Error("DOT missing edges")
	}
}

// --- Panic on duplicate ---

func TestT_PanicOnDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate transition")
		}
	}()
	machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.T(Idle, Start, Done), // duplicate (Idle, Start)
	)
}

func TestTG_PanicOnDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate guarded transition")
		}
	}()
	g := func(ctx context.Context) bool { return true }
	machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.TG(Idle, Start, Done, g), // duplicate (Idle, Start)
	)
}

// --- Concurrency ---

func TestSend_Concurrent(t *testing.T) {
	// Multiple goroutines reading State() concurrently with sends.
	m := machina.New[S, E](Idle,
		machina.T(Idle, Start, Running),
		machina.T(Running, Stop, Done),
	)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.State()
			_ = m.Can(Start)
		}()
	}
	m.Send(ctx, Start)
	m.Send(ctx, Stop)
	wg.Wait()
	if m.State() != Done {
		t.Fatalf("want Done, got %v", m.State())
	}
}
