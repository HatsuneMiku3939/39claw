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

	admission, err := coordinator.Admit("thread-1", func() { executed = append(executed, 1) }, nil)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if !admission.ExecuteNow || admission.Queued || admission.Position != 0 {
		t.Fatalf("first admission = %#v, want immediate execution", admission)
	}

	for i := range []int{2, 3} {
		value := i + 2
		admission, err = coordinator.Admit("thread-1", func() { executed = append(executed, value) }, nil)
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

	for range []int{2, 3} {
		next, ok := coordinator.Complete("thread-1")
		if !ok || next == nil {
			t.Fatalf("Complete() returned ok=%t, next nil=%t; want queued work", ok, next == nil)
		}
		next()
	}

	if !slices.Equal(executed, []int{2, 3}) {
		t.Fatalf("executed queued work = %v, want %v", executed, []int{2, 3})
	}

	next, ok := coordinator.Complete("thread-1")
	if ok || next != nil {
		t.Fatalf("final Complete() returned ok=%t, next nil=%t; want (nil, false)", ok, next == nil)
	}

	if snapshot := coordinator.Snapshot("thread-1"); snapshot != (app.QueueSnapshot{}) {
		t.Fatalf("final Snapshot() = %#v, want empty snapshot", snapshot)
	}
}

func TestQueueCoordinatorRejectsWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	coordinator := thread.NewQueueCoordinator()

	firstAdmission, err := coordinator.Admit("thread-1", func() {}, nil)
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if !firstAdmission.ExecuteNow {
		t.Fatalf("first admission = %#v, want immediate execution", firstAdmission)
	}

	for position := 1; position <= 5; position++ {
		admission, err := coordinator.Admit("thread-1", func() {}, nil)
		if err != nil {
			t.Fatalf("Admit() error = %v", err)
		}
		if !admission.Queued || admission.Position != position {
			t.Fatalf("queued admission = %#v, want position %d", admission, position)
		}
	}

	_, err = coordinator.Admit("thread-1", func() {}, nil)
	if !errors.Is(err, app.ErrExecutionQueueFull) {
		t.Fatalf("Admit() error = %v, want %v", err, app.ErrExecutionQueueFull)
	}
}

func TestQueueCoordinatorDropQueuedLeavesRunningEntry(t *testing.T) {
	t.Parallel()

	coordinator := thread.NewQueueCoordinator()
	dropped := 0

	if _, err := coordinator.Admit("thread-1", func() {}, nil); err != nil {
		t.Fatalf("Admit() first error = %v", err)
	}
	if _, err := coordinator.Admit("thread-1", func() {}, func() { dropped++ }); err != nil {
		t.Fatalf("Admit() second error = %v", err)
	}
	if _, err := coordinator.Admit("thread-1", func() {}, func() { dropped++ }); err != nil {
		t.Fatalf("Admit() third error = %v", err)
	}

	if droppedCount := coordinator.DropQueued("thread-1"); droppedCount != 2 {
		t.Fatalf("DropQueued() = %d, want %d", droppedCount, 2)
	}

	if dropped != 2 {
		t.Fatalf("drop callback count = %d, want %d", dropped, 2)
	}

	if snapshot := coordinator.Snapshot("thread-1"); snapshot != (app.QueueSnapshot{InFlight: true}) {
		t.Fatalf("Snapshot() = %#v, want in-flight with empty queue", snapshot)
	}

	if next, ok := coordinator.Complete("thread-1"); ok || next != nil {
		t.Fatalf("Complete() = (nil:%t, ok:%t), want (nil:true, ok:false)", next == nil, ok)
	}
}
