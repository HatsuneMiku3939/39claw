package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Thread struct {
	executor *executor
	options  ThreadOptions

	mu sync.RWMutex
	id string
}

type StreamedTurn struct {
	events <-chan Event
	errCh  <-chan error
}

func newThread(executor *executor, options ThreadOptions, id string) *Thread {
	return &Thread{
		executor: executor,
		options:  options,
		id:       id,
	}
}

func (t *Thread) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.id
}

func (t *Thread) Run(ctx context.Context, input Input) (Turn, error) {
	stream, err := t.RunStreamed(ctx, input)
	if err != nil {
		return Turn{}, err
	}

	turn := Turn{}
	var turnFailure *ThreadError
	var streamFailure string

	for event := range stream.Events() {
		switch event.Type {
		case "item.completed":
			if event.Item == nil {
				continue
			}

			turn.Items = append(turn.Items, *event.Item)
			if event.Item.Type == "agent_message" {
				turn.FinalResponse = event.Item.Text
			}
		case "turn.completed":
			turn.Usage = event.Usage
		case "turn.failed":
			turnFailure = event.Error
		case "error":
			streamFailure = event.Message
		}
	}

	if err := stream.Wait(); err != nil {
		return Turn{}, err
	}

	if turnFailure != nil {
		return Turn{}, errors.New(turnFailure.Message)
	}

	if streamFailure != "" {
		return Turn{}, errors.New(streamFailure)
	}

	return turn, nil
}

func (t *Thread) RunStreamed(ctx context.Context, input Input) (*StreamedTurn, error) {
	normalized, err := input.normalize()
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}

	events := make(chan Event)
	errCh := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errCh)

		err := t.executor.run(ctx, execRequest{
			prompt:        normalized.Prompt,
			images:        normalized.Images,
			threadID:      t.ID(),
			threadOptions: t.options,
		}, func(line string) error {
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				return fmt.Errorf("parse codex event: %w", err)
			}

			if event.Type == "thread.started" && event.ThreadID != "" {
				t.setID(event.ThreadID)
			}

			select {
			case events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		errCh <- err
	}()

	return &StreamedTurn{
		events: events,
		errCh:  errCh,
	}, nil
}

func (s *StreamedTurn) Events() <-chan Event {
	return s.events
}

func (s *StreamedTurn) Wait() error {
	return <-s.errCh
}

func (t *Thread) setID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.id = id
}
