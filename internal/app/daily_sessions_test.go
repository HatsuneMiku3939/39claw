package app_test

import (
	"context"
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestResolveActiveDailySessionReturnsExistingSession(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	existing, err := store.CreateDailySession(context.Background(), app.DailySession{
		LocalDate:        "2026-04-11",
		Generation:       1,
		LogicalThreadKey: "2026-04-11#1",
		ActivationReason: app.DailySessionActivationAutomatic,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	session, err := app.ResolveActiveDailySession(context.Background(), store, "2026-04-11")
	if err != nil {
		t.Fatalf("ResolveActiveDailySession() error = %v", err)
	}

	if session != existing {
		t.Fatalf("ResolveActiveDailySession() = %#v, want %#v", session, existing)
	}
}

func TestResolveActiveDailySessionCreatesSessionFromPreviousDay(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	previous, err := store.CreateDailySession(context.Background(), app.DailySession{
		LocalDate:        "2026-04-10",
		Generation:       2,
		LogicalThreadKey: "2026-04-10#2",
		ActivationReason: app.DailySessionActivationClear,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	session, err := app.ResolveActiveDailySession(context.Background(), store, "2026-04-11")
	if err != nil {
		t.Fatalf("ResolveActiveDailySession() error = %v", err)
	}

	if session.LocalDate != "2026-04-11" {
		t.Fatalf("LocalDate = %q, want %q", session.LocalDate, "2026-04-11")
	}
	if session.Generation != 1 {
		t.Fatalf("Generation = %d, want %d", session.Generation, 1)
	}
	if session.LogicalThreadKey != "2026-04-11#1" {
		t.Fatalf("LogicalThreadKey = %q, want %q", session.LogicalThreadKey, "2026-04-11#1")
	}
	if session.PreviousLogicalThreadKey != previous.LogicalThreadKey {
		t.Fatalf("PreviousLogicalThreadKey = %q, want %q", session.PreviousLogicalThreadKey, previous.LogicalThreadKey)
	}
	if session.ActivationReason != app.DailySessionActivationAutomatic {
		t.Fatalf("ActivationReason = %q, want %q", session.ActivationReason, app.DailySessionActivationAutomatic)
	}
	if !session.IsActive {
		t.Fatal("IsActive = false, want true")
	}
}

func TestParseDailyLogicalKey(t *testing.T) {
	t.Parallel()

	localDate, generation, err := app.ParseDailyLogicalKey(" 2026-04-11#2 ")
	if err != nil {
		t.Fatalf("ParseDailyLogicalKey() error = %v", err)
	}
	if localDate != "2026-04-11" {
		t.Fatalf("localDate = %q, want %q", localDate, "2026-04-11")
	}
	if generation != 2 {
		t.Fatalf("generation = %d, want %d", generation, 2)
	}
}

func TestParseDailyLogicalKeyRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"",
		"2026-04-11",
		"#1",
		"2026-04-11#",
		"2026-04-11#0",
		"2026-04-11#-1",
		"2026-04-11#x",
	}

	for _, logicalKey := range testCases {
		logicalKey := logicalKey
		t.Run(logicalKey, func(t *testing.T) {
			t.Parallel()

			_, _, err := app.ParseDailyLogicalKey(logicalKey)
			if err == nil {
				t.Fatalf("ParseDailyLogicalKey(%q) error = nil, want invalid key error", logicalKey)
			}
		})
	}
}
