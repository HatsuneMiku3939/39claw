package discord

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestMapMessageCreate(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, time.April, 10, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		event       *discordgo.MessageCreate
		wantOK      bool
		wantContent string
	}{
		{
			name: "guild mention maps request",
			event: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "message-1",
					ChannelID: "channel-1",
					GuildID:   "guild-1",
					Content:   "<@bot-user> hello there",
					Author:    &discordgo.User{ID: "user-1"},
					Mentions: []*discordgo.User{
						{ID: "bot-user"},
					},
					Timestamp: timestamp,
				},
			},
			wantOK:      true,
			wantContent: "hello there",
		},
		{
			name: "direct message maps request without mention",
			event: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "message-2",
					ChannelID: "dm-channel-1",
					Content:   "hello from dm",
					Author:    &discordgo.User{ID: "user-1"},
					Timestamp: timestamp,
				},
			},
			wantOK:      true,
			wantContent: "hello from dm",
		},
		{
			name: "guild chatter without mention is ignored",
			event: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "message-3",
					ChannelID: "channel-1",
					GuildID:   "guild-1",
					Content:   "just chatting",
					Author:    &discordgo.User{ID: "user-1"},
					Timestamp: timestamp,
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, ok := mapMessageCreate("bot-user", tt.event)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !tt.wantOK {
				return
			}

			if request.Content != tt.wantContent {
				t.Fatalf("content = %q, want %q", request.Content, tt.wantContent)
			}

			if !request.Mentioned {
				t.Fatal("Mentioned = false, want true")
			}

			if !request.ReceivedAt.Equal(timestamp) {
				t.Fatalf("received at = %v, want %v", request.ReceivedAt, timestamp)
			}
		})
	}
}
