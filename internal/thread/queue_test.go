package thread_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
)

func TestQueueCoordinatorCompletesQueuedWorkInOrder(t *testing.T) {
	t.Parallel()

	coordinator := thread.NewQueueCoordinator()
	var executed []int

	admission, err := coordinator.Admit("thread-1", func() { executed = append(executed, 1) })
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if !admission.ExecuteNow || admission.Queued || admission.Position != 0 {
		t.Fatalf("first admission = %#v, want immediate execution", admission)
	}

	for i := range []int{2, 3} {
		value := i + 2
		admission, err = coordinator.Admit("thread-1", func() { executed = append(executed, value) })
		if err != nil {
			t.Fatalf("Admit() error = %v", err)
		}
		if !admission.Queued || admission.ExecuteNow || admission.Position != value-1 {
			t.Fatalf("queued admission %d = %#v", value, admission)
		}
	}

	snapshot := coordinator.Snapshot("thread-1")
	if snapshot != (app.QueueSnapshot{InFlight: true, Queued: 2}) {
		t.Fatalf("Snapshot() = %#v, want %#v", snapshot, app.QueueSnapshot{InFlight: true, Queued: 2})
	}

	for _, want := range []int{2, 3} {
		next, ok := coordinator.Complete("thread-1")
		if !ok || next == nil {
			t.Fatalf("Complete() = (%v, %t), want queued work", next, ok)
		}
		next()
	}

	if !slices.Equal(executed, []int{2, 3}) {
		t.Fatalf("executed queued work = %v, want %v", executed, []int{2, 3})
	}

	next, ok := coordinator.Complete("thread-1")
	if ok || next != nil {
		t.Fatalf("final Complete() = (%v, %t), want (nil, false)", next, ok)
	}

	if snapshot := coordinator.Snapshot("thread-1"); snapshot != (app.QueueSnapshot{}) {
		t.Fatalf("final Snapshot() = %#v, want empty snapshot", snapshot)
	}
}

func TestQueueCoordinatorRejectsWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	coordinator := thread.NewQueueCoordinator()

	firstAdmission, err := coordinator.Admit("thread-1", func() {})
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if !firstAdmission.ExecuteNow {
		t.Fatalf("first admission = %#v, want immediate execution", firstAdmission)
	}

	for position := 1; position <= 5; position++ {
		admission, err := coordinator.Admit("thread-1", func() {})
		if err != nil {
			t.Fatalf("Admit() error = %v", err)
		}
		if !admission.Queued || admission.Position != position {
			t.Fatalf("queued admission = %#v, want position %d", admission, position)
		}
	}

	_, err = coordinator.Admit("thread-1", func() {})
	if !errors.Is(err, app.ErrExecutionQueueFull) {
		t.Fatalf("Admit() error = %v, want %v", err, app.ErrExecutionQueueFull)
	}
}
