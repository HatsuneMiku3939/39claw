package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestMapInteractionCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event *discordgo.InteractionCreate
		want  commandRequest
		ok    bool
	}{
		{
			name: "maps configured root command action and task fields",
			event: &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Type: discordgo.InteractionApplicationCommand,
					Data: discordgo.ApplicationCommandInteractionData{
						Name: "release",
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{Name: optionTaskID, Type: discordgo.ApplicationCommandOptionString, Value: " task-1 "},
							{Name: optionAction, Type: discordgo.ApplicationCommandOptionString, Value: actionTaskSwitch},
							{Name: optionTaskName, Type: discordgo.ApplicationCommandOptionString, Value: " Release work "},
						},
					},
					Member: &discordgo.Member{
						User: &discordgo.User{ID: "user-1"},
					},
				},
			},
			want: commandRequest{
				Name:     "release",
				Action:   actionTaskSwitch,
				UserID:   "user-1",
				TaskName: "Release work",
				TaskID:   "task-1",
			},
			ok: true,
		},
		{
			name: "falls back to interaction user",
			event: &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Type: discordgo.InteractionApplicationCommand,
					Data: discordgo.ApplicationCommandInteractionData{
						Name: "daily",
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{Name: optionAction, Type: discordgo.ApplicationCommandOptionString, Value: actionHelp},
						},
					},
					User: &discordgo.User{ID: "user-2"},
				},
			},
			want: commandRequest{
				Name:   "daily",
				Action: actionHelp,
				UserID: "user-2",
			},
			ok: true,
		},
		{
			name: "maps task reset action without task selector fields",
			event: &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Type: discordgo.InteractionApplicationCommand,
					Data: discordgo.ApplicationCommandInteractionData{
						Name: "release",
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{Name: optionAction, Type: discordgo.ApplicationCommandOptionString, Value: actionTaskResetContext},
						},
					},
					Member: &discordgo.Member{
						User: &discordgo.User{ID: "user-3"},
					},
				},
			},
			want: commandRequest{
				Name:   "release",
				Action: actionTaskResetContext,
				UserID: "user-3",
			},
			ok: true,
		},
		{
			name: "keeps request when action is missing",
			event: &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Type: discordgo.InteractionApplicationCommand,
					Data: discordgo.ApplicationCommandInteractionData{
						Name: "release",
					},
					Member: &discordgo.Member{
						User: &discordgo.User{ID: "user-1"},
					},
				},
			},
			want: commandRequest{
				Name:   "release",
				UserID: "user-1",
			},
			ok: true,
		},
		{
			name: "rejects interaction without user",
			event: &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Type: discordgo.InteractionApplicationCommand,
					Data: discordgo.ApplicationCommandInteractionData{
						Name: "release",
					},
				},
			},
			ok: false,
		},
		{
			name:  "rejects nil event",
			event: nil,
			ok:    false,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := mapInteractionCommand(tt.event)
			if ok != tt.ok {
				t.Fatalf("mapInteractionCommand() ok = %t, want %t", ok, tt.ok)
			}

			if !tt.ok {
				return
			}

			if got != tt.want {
				t.Fatalf("mapInteractionCommand() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
