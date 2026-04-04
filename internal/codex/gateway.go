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

func (g *Gateway) RunTurn(ctx context.Context, threadID string, prompt string) (app.RunTurnResult, error) {
	if g.client == nil {
		return app.RunTurnResult{}, errors.New("codex client must not be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return app.RunTurnResult{}, errors.New("prompt must not be empty")
	}

	thread := g.client.StartThread(g.threadOptions)
	if threadID != "" {
		thread = g.client.ResumeThread(threadID, g.threadOptions)
	}

	turn, err := thread.Run(ctx, TextInput(prompt))
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
