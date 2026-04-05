package codex

import (
	"context"
	"errors"
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

	turn, err := thread.Run(ctx, codexInput)
	if err != nil {
		return app.RunTurnResult{}, err
	}

	result := app.RunTurnResult{
		ThreadID:     thread.ID(),
		ResponseText: turn.FinalResponse,
	}

	if turn.Usage != nil {
		result.Usage = &app.TokenUsage{
			InputTokens:       turn.Usage.InputTokens,
			CachedInputTokens: turn.Usage.CachedInputTokens,
			OutputTokens:      turn.Usage.OutputTokens,
		}
	}

	return result, nil
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
