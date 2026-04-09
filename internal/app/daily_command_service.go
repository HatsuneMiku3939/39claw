package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

type DailyCommandService interface {
	Clear(ctx context.Context, userID string, receivedAt time.Time) (MessageResponse, error)
}

type DailyCommandServiceDependencies struct {
	CommandName string
	Timezone    *time.Location
	Store       ThreadStore
	Coordinator QueueCoordinator
}

type DefaultDailyCommandService struct {
	commands    commandSurface
	timezone    *time.Location
	store       ThreadStore
	coordinator QueueCoordinator
}

func NewDailyCommandService(deps DailyCommandServiceDependencies) (*DefaultDailyCommandService, error) {
	commandName := strings.TrimSpace(deps.CommandName)
	if commandName == "" {
		return nil, errors.New("command name must not be empty")
	}

	if deps.Timezone == nil {
		return nil, errors.New("timezone must not be nil")
	}

	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	if deps.Coordinator == nil {
		return nil, errors.New("queue coordinator must not be nil")
	}

	return &DefaultDailyCommandService{
		commands:    newCommandSurface(commandName),
		timezone:    deps.Timezone,
		store:       deps.Store,
		coordinator: deps.Coordinator,
	}, nil
}

func (s *DefaultDailyCommandService) Clear(ctx context.Context, _ string, receivedAt time.Time) (MessageResponse, error) {
	localDate := receivedAt.In(s.timezone).Format(time.DateOnly)

	active, err := ResolveActiveDailySession(ctx, s.store, localDate)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("resolve active daily session: %w", err)
	}

	snapshot := s.coordinator.Snapshot(buildExecutionKey(config.ModeDaily, active.LogicalThreadKey))
	if snapshot.InFlight || snapshot.Queued > 0 {
		return dailyCommandResponse(
			fmt.Sprintf(
				"Today's shared daily conversation is still busy. Wait for pending replies to finish, then retry `/%s action:clear`.",
				s.commands.commandName,
			),
		), nil
	}

	next, err := s.store.RotateDailySession(ctx, localDate, DailySessionActivationClear)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("rotate daily session: %w", err)
	}

	return dailyCommandResponse(
		fmt.Sprintf(
			"Started a fresh shared daily conversation for today. The active generation is now `%s`.",
			next.LogicalThreadKey,
		),
	), nil
}

func dailyCommandResponse(text string) MessageResponse {
	return MessageResponse{
		Text:      text,
		Ephemeral: true,
	}
}
