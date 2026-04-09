package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestDailyCommandServiceClearCreatesAndRotatesTodaySession(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	store := &memoryThreadStore{}
	coordinator := &stubQueueCoordinator{}
	service, err := app.NewDailyCommandService(app.DailyCommandServiceDependencies{
		CommandName: "release",
		Timezone:    tokyo,
		Store:       store,
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewDailyCommandService() error = %v", err)
	}

	response, err := service.Clear(
		context.Background(),
		"user-1",
		time.Date(2026, time.April, 5, 10, 0, 0, 0, tokyo),
	)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if !response.Ephemeral {
		t.Fatal("Ephemeral = false, want true")
	}

	active, ok, err := store.GetActiveDailySession(context.Background(), "2026-04-05")
	if err != nil {
		t.Fatalf("GetActiveDailySession() error = %v", err)
	}
	if !ok {
		t.Fatal("GetActiveDailySession() ok = false, want true")
	}
	if active.LogicalThreadKey != "2026-04-05#2" {
		t.Fatalf("LogicalThreadKey = %q, want %q", active.LogicalThreadKey, "2026-04-05#2")
	}
	if active.PreviousLogicalThreadKey != "2026-04-05#1" {
		t.Fatalf("PreviousLogicalThreadKey = %q, want %q", active.PreviousLogicalThreadKey, "2026-04-05#1")
	}
}

func TestDailyCommandServiceClearRejectsBusyGeneration(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	store := &memoryThreadStore{}
	if _, err := store.CreateDailySession(context.Background(), app.DailySession{
		LocalDate:        "2026-04-05",
		Generation:       1,
		LogicalThreadKey: "2026-04-05#1",
		ActivationReason: app.DailySessionActivationAutomatic,
		IsActive:         true,
	}); err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	service, err := app.NewDailyCommandService(app.DailyCommandServiceDependencies{
		CommandName: "release",
		Timezone:    tokyo,
		Store:       store,
		Coordinator: &stubQueueCoordinator{
			snapshot: app.QueueSnapshot{InFlight: true, Queued: 1},
		},
	})
	if err != nil {
		t.Fatalf("NewDailyCommandService() error = %v", err)
	}

	response, err := service.Clear(
		context.Background(),
		"user-1",
		time.Date(2026, time.April, 5, 10, 0, 0, 0, tokyo),
	)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if !response.Ephemeral {
		t.Fatal("Ephemeral = false, want true")
	}

	active, ok, err := store.GetActiveDailySession(context.Background(), "2026-04-05")
	if err != nil {
		t.Fatalf("GetActiveDailySession() error = %v", err)
	}
	if !ok {
		t.Fatal("GetActiveDailySession() ok = false, want true")
	}
	if active.LogicalThreadKey != "2026-04-05#1" {
		t.Fatalf("LogicalThreadKey = %q, want %q", active.LogicalThreadKey, "2026-04-05#1")
	}
}
