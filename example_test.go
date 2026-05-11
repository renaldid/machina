package machina_test

import (
	"context"
	"fmt"

	"github.com/renaldid/machina"
)

func ExampleFSM_Send() {
	type State = string
	type Event = string

	m := machina.New[State, Event]("idle",
		machina.T("idle", "start", "running"),
		machina.T("running", "pause", "paused"),
		machina.T("paused", "resume", "running"),
		machina.T("running", "stop", "done"),
	)

	ctx := context.Background()
	_ = m.Send(ctx, "start")
	_ = m.Send(ctx, "pause")
	_ = m.Send(ctx, "resume")
	_ = m.Send(ctx, "stop")
	fmt.Println(m.State())
	// Output: done
}

func ExampleFSM_Can() {
	type State = string
	type Event = string

	m := machina.New[State, Event]("locked",
		machina.T("locked", "insert_coin", "unlocked"),
		machina.T("unlocked", "push", "locked"),
		machina.T("unlocked", "insert_coin", "unlocked"),
	)

	fmt.Println(m.Can("insert_coin"))
	fmt.Println(m.Can("push"))
	// Output:
	// true
	// false
}

func ExampleTG() {
	type State = string
	type Event = string

	admin := false
	m := machina.New[State, Event]("pending",
		machina.TG("pending", "approve", "approved", func(ctx context.Context) bool {
			return admin
		}),
	)

	ctx := context.Background()
	err := m.Send(ctx, "approve")
	fmt.Println(err != nil)

	admin = true
	err = m.Send(ctx, "approve")
	fmt.Println(err)
	fmt.Println(m.State())
	// Output:
	// true
	// <nil>
	// approved
}
