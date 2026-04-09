package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	DailySessionActivationAutomatic = "automatic"
	DailySessionActivationClear     = "clear"
)

func BuildDailyLogicalKey(localDate string, generation int) string {
	return fmt.Sprintf("%s#%d", strings.TrimSpace(localDate), generation)
}

func ResolveActiveDailySession(ctx context.Context, store ThreadStore, localDate string) (DailySession, error) {
	active, ok, err := store.GetActiveDailySession(ctx, localDate)
	if err != nil {
		return DailySession{}, fmt.Errorf("load active daily session: %w", err)
	}

	if ok {
		return active, nil
	}

	previousLogicalKey := ""
	previous, ok, err := store.GetLatestDailySessionBefore(ctx, localDate)
	if err != nil {
		return DailySession{}, fmt.Errorf("load previous daily session: %w", err)
	}

	if ok {
		previousLogicalKey = previous.LogicalThreadKey
	}

	session, err := store.CreateDailySession(ctx, DailySession{
		LocalDate:                localDate,
		Generation:               1,
		LogicalThreadKey:         BuildDailyLogicalKey(localDate, 1),
		PreviousLogicalThreadKey: previousLogicalKey,
		ActivationReason:         DailySessionActivationAutomatic,
		IsActive:                 true,
	})
	if err != nil {
		return DailySession{}, fmt.Errorf("create daily session: %w", err)
	}

	return session, nil
}

func ParseDailyLogicalKey(logicalKey string) (string, int, error) {
	localDate, generationText, ok := strings.Cut(strings.TrimSpace(logicalKey), "#")
	if !ok || localDate == "" || generationText == "" {
		return "", 0, fmt.Errorf("invalid daily logical key %q", logicalKey)
	}

	generation, err := strconv.Atoi(generationText)
	if err != nil || generation < 1 {
		return "", 0, fmt.Errorf("invalid daily logical key %q", logicalKey)
	}

	return localDate, generation, nil
}
