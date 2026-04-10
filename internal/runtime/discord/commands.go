package discord

import (
	"fmt"
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/bwmarrin/discordgo"
)

const (
	internalErrorMessage      = "Something went wrong while handling that request. Please retry in a moment."
	imageDownloadErrorMessage = "I couldn't download one of the image attachments. Please retry in a moment."
)

func registeredCommands(cfg config.Config) []*discordgo.ApplicationCommand {
	actionChoices := []*discordgo.ApplicationCommandOptionChoice{
		{
			Name:  actionHelp,
			Value: actionHelp,
		},
	}

	if cfg.Mode == config.ModeDaily {
		actionChoices = append(
			actionChoices,
			&discordgo.ApplicationCommandOptionChoice{Name: actionClear, Value: actionClear},
		)
	}

	if cfg.Mode == config.ModeTask {
		actionChoices = append(
			actionChoices,
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskCurrent, Value: actionTaskCurrent},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskList, Value: actionTaskList},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskNew, Value: actionTaskNew},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskSwitch, Value: actionTaskSwitch},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskClose, Value: actionTaskClose},
		)
	}

	options := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionAction,
			Description: "Choose what this bot instance should do.",
			Required:    true,
			Choices:     actionChoices,
		},
	}

	if cfg.Mode == config.ModeTask {
		options = append(
			options,
			&discordgo.ApplicationCommandOption{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        optionTaskName,
				Description: "Task name for task-new or task-switch.",
				Required:    false,
			},
			&discordgo.ApplicationCommandOption{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        optionTaskID,
				Description: "Task ID for task-switch when a name is ambiguous, or for task-close.",
				Required:    false,
			},
		)
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        cfg.DiscordCommandName,
			Description: "Show help and run task actions for this bot instance.",
			Options:     options,
		},
	}
}

func helpResponse(commandName string, mode config.Mode) app.MessageResponse {
	lines := []string{
		fmt.Sprintf("Command: /%s", commandName),
		fmt.Sprintf("Mode: %s", mode),
		"- Mention the bot in a message to start or continue the conversation.",
		"Available actions:",
		fmt.Sprintf("- `/%s action:%s` shows this help message.", commandName, actionHelp),
	}

	if mode == config.ModeDaily {
		lines = append(lines,
			fmt.Sprintf("- `/%s action:%s` starts a fresh shared daily generation for today when the current one is idle.", commandName, actionClear),
		)
	}

	if mode == config.ModeTask {
		lines = append(lines,
			fmt.Sprintf("- `/%s action:%s` shows the active task.", commandName, actionTaskCurrent),
			fmt.Sprintf("- `/%s action:%s` lists open tasks.", commandName, actionTaskList),
			fmt.Sprintf("- `/%s action:%s task_name:<name>` creates and activates a task.", commandName, actionTaskNew),
			fmt.Sprintf("- `/%s action:%s task_name:<name>` changes the active task and falls back to `task_id` when the name is ambiguous.", commandName, actionTaskSwitch),
			fmt.Sprintf("- `/%s action:%s task_id:<id>` closes a task.", commandName, actionTaskClose),
		)
	}

	return app.MessageResponse{
		Text: strings.Join(lines, "\n"),
	}
}

func taskUnavailableDailyMode(commandName string) string {
	return fmt.Sprintf(
		"Task actions are not available in this daily-mode bot. Use `/%s action:%s` or mention the bot to continue today's conversation.",
		commandName,
		actionHelp,
	)
}

func unsupportedActionText(commandName string, mode config.Mode) string {
	if mode != config.ModeTask {
		return fmt.Sprintf("Unsupported action. Use `/%s action:%s` or `/%s action:%s`.", commandName, actionHelp, commandName, actionClear)
	}

	return fmt.Sprintf(
		"Unsupported action. Use `/%s action:%s`, `/%s action:%s`, `/%s action:%s`, `/%s action:%s task_name:<name>`, `/%s action:%s task_name:<name>`, or `/%s action:%s task_id:<id>`.",
		commandName,
		actionHelp,
		commandName,
		actionTaskCurrent,
		commandName,
		actionTaskList,
		commandName,
		actionTaskNew,
		commandName,
		actionTaskSwitch,
		commandName,
		actionTaskClose,
	)
}
