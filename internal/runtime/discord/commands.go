package discord

import (
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/bwmarrin/discordgo"
)

const (
	internalErrorMessage      = "Something went wrong while handling that request. Please retry in a moment."
	taskUnavailableDailyMode  = "Task commands are not available in this daily-mode bot. Mention the bot to continue today's conversation."
	unsupportedTaskActionText = "Unsupported task command. Use `/task current`, `/task list`, `/task new`, `/task switch`, or `/task close`."
)

func registeredCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        commandHelp,
			Description: "Show the commands available in this bot instance.",
		},
		{
			Name:        commandTask,
			Description: "Manage the active task workflow.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        taskActionCurrent,
					Description: "Show the current active task.",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        taskActionList,
					Description: "List open tasks.",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        taskActionNew,
					Description: "Create a new task and make it active.",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "The task name to create.",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        taskActionSwitch,
					Description: "Switch the active task.",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "id",
							Description: "The task ID to activate.",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        taskActionClose,
					Description: "Close a task by ID.",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "id",
							Description: "The task ID to close.",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

func helpResponse(mode config.Mode) app.MessageResponse {
	lines := []string{
		"Available commands:",
		"- Mention the bot in a message to start or continue the conversation.",
		"- `/help` shows this help message.",
	}

	if mode == config.ModeTask {
		lines = append(lines,
			"- `/task current` shows the active task.",
			"- `/task list` shows open tasks.",
			"- `/task new <name>` creates and activates a task.",
			"- `/task switch <id>` changes the active task.",
			"- `/task close <id>` closes a task.",
		)
	}

	return app.MessageResponse{
		Text: strings.Join(lines, "\n"),
	}
}
