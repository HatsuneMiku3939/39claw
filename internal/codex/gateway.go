package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

type GatewayOptions struct {
	ThreadOptions ThreadOptions
}

type Gateway struct {
	client        *Client
	threadOptions ThreadOptions
}

const agentMessageItemType = "agent_message"

func NewGateway(client *Client, options GatewayOptions) *Gateway {
	return &Gateway{
		client:        client,
		threadOptions: options.ThreadOptions,
	}
}

func (g *Gateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	if g.client == nil {
		return app.RunTurnResult{}, errors.New("codex client must not be nil")
	}

	codexInput, err := buildGatewayInput(input)
	if err != nil {
		return app.RunTurnResult{}, err
	}

	threadOptions := g.threadOptions
	if strings.TrimSpace(input.WorkingDirectory) != "" {
		threadOptions.WorkingDirectory = input.WorkingDirectory
	}

	thread := g.client.StartThread(threadOptions)
	if threadID != "" {
		thread = g.client.ResumeThread(threadID, threadOptions)
	}

	stream, err := thread.RunStreamed(ctx, codexInput)
	if err != nil {
		return app.RunTurnResult{}, err
	}

	result := app.RunTurnResult{}
	var turnFailure *ThreadError
	var streamFailure string
	responseStarted := false
	lastProgressText := ""

	for event := range stream.Events() {
		switch event.Type {
		case "item.started", "item.updated", "item.completed":
			if event.Item == nil {
				continue
			}

			if event.Item.Type == agentMessageItemType {
				result.ResponseText = event.Item.Text
				if strings.TrimSpace(event.Item.Text) != "" {
					responseStarted = true
				}
			}

			progressText := progressTextForEvent(event, responseStarted)
			if input.ProgressSink != nil && strings.TrimSpace(progressText) != "" && progressText != lastProgressText {
				if err := input.ProgressSink.Deliver(ctx, app.MessageProgress{Text: progressText}); err != nil {
					ignoreProgressDeliveryError(err)
				}
				lastProgressText = progressText
			}
		case "turn.completed":
			if event.Usage != nil {
				result.Usage = &app.TokenUsage{
					InputTokens:       event.Usage.InputTokens,
					CachedInputTokens: event.Usage.CachedInputTokens,
					OutputTokens:      event.Usage.OutputTokens,
				}
			}
		case "turn.failed":
			turnFailure = event.Error
		case "error":
			streamFailure = event.Message
		}
	}

	if err := stream.Wait(); err != nil {
		return app.RunTurnResult{}, err
	}

	if turnFailure != nil {
		return app.RunTurnResult{}, errors.New(turnFailure.Message)
	}

	if streamFailure != "" {
		return app.RunTurnResult{}, errors.New(streamFailure)
	}

	result.ThreadID = thread.ID()
	return result, nil
}

func progressTextForEvent(event Event, responseStarted bool) string {
	if event.Item == nil {
		return ""
	}

	item := event.Item
	if item.Type == agentMessageItemType {
		return item.Text
	}

	if responseStarted {
		return ""
	}

	switch item.Type {
	case "command_execution":
		command := strings.TrimSpace(item.Command)
		if command == "" {
			return "Running a command..."
		}

		return fmt.Sprintf("Running command: `%s`", command)
	case "mcp_tool_call":
		if item.Server != "" && item.Tool != "" {
			return fmt.Sprintf("Using tool: `%s/%s`", item.Server, item.Tool)
		}

		if item.Tool != "" {
			return fmt.Sprintf("Using tool: `%s`", item.Tool)
		}

		return "Using a tool..."
	case "web_search":
		query := strings.TrimSpace(item.Query)
		if query == "" {
			return "Searching the web..."
		}

		return fmt.Sprintf("Searching the web: `%s`", query)
	case "file_change":
		return "Applying file changes..."
	default:
		return ""
	}
}

func ignoreProgressDeliveryError(err error) {
	_ = err
}

func buildGatewayInput(input app.CodexTurnInput) (Input, error) {
	parts := make([]InputPart, 0, len(input.ImagePaths)+1)
	if prompt := strings.TrimSpace(input.Prompt); prompt != "" {
		parts = append(parts, TextPart(prompt))
	}

	for _, imagePath := range input.ImagePaths {
		if strings.TrimSpace(imagePath) == "" {
			continue
		}

		parts = append(parts, LocalImagePart(imagePath))
	}

	if len(parts) == 0 {
		return Input{}, errors.New("turn input must include text or at least one image")
	}

	return MultiPartInput(parts...), nil
}
