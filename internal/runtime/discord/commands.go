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

	if cfg.Mode == config.ModeJournal {
		actionChoices = append(
			actionChoices,
			&discordgo.ApplicationCommandOptionChoice{Name: actionClear, Value: actionClear},
		)
	}

	if cfg.Mode == config.ModeThread {
		actionChoices = append(
			actionChoices,
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskCurrent, Value: actionTaskCurrent},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskList, Value: actionTaskList},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskNew, Value: actionTaskNew},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskSwitch, Value: actionTaskSwitch},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskClose, Value: actionTaskClose},
			&discordgo.ApplicationCommandOptionChoice{Name: actionTaskResetContext, Value: actionTaskResetContext},
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

	if cfg.Mode == config.ModeThread {
		options = append(
			options,
			&discordgo.ApplicationCommandOption{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        optionTaskName,
				Description: "Slug-style task name for task-new, task-switch, or task-close.",
				Required:    false,
			},
			&discordgo.ApplicationCommandOption{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        optionTaskID,
				Description: "Legacy selector for task-switch or task-close when needed.",
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

	if mode == config.ModeJournal {
		lines = append(lines,
			fmt.Sprintf("- `/%s action:%s` starts a fresh shared journal generation for today when the current one is idle.", commandName, actionClear),
		)
	}

	if mode == config.ModeThread {
		lines = append(lines,
			fmt.Sprintf("- `/%s action:%s` shows the active task.", commandName, actionTaskCurrent),
			fmt.Sprintf("- `/%s action:%s` lists open tasks.", commandName, actionTaskList),
			fmt.Sprintf("- `/%s action:%s task_name:<name>` creates and activates a task. %s", commandName, actionTaskNew, app.TaskNameRulesDescription),
			fmt.Sprintf("- `/%s action:%s task_name:<name>` changes the active task. `task_id` remains available only for legacy selection cases.", commandName, actionTaskSwitch),
			fmt.Sprintf("- `/%s action:%s task_name:<name>` closes a task. `task_id` remains available only for legacy selection cases.", commandName, actionTaskClose),
			fmt.Sprintf("- `/%s action:%s` resets only the saved Codex conversation continuity for the active task.", commandName, actionTaskResetContext),
			"- Normal thread-mode messages can start with `task:<name>` to route just that one message without changing the active task.",
		)
	}

	return app.MessageResponse{
		Text: strings.Join(lines, "\n"),
	}
}

func taskUnavailableJournalMode(commandName string) string {
	return fmt.Sprintf(
		"Task actions are not available in this journal-mode bot. Use `/%s action:%s` or mention the bot to continue today's conversation.",
		commandName,
		actionHelp,
	)
}

func unsupportedActionText(commandName string, mode config.Mode) string {
	if mode != config.ModeThread {
		return fmt.Sprintf("Unsupported action. Use `/%s action:%s` or `/%s action:%s`.", commandName, actionHelp, commandName, actionClear)
	}

	return fmt.Sprintf(
		"Unsupported action. Use `/%s action:%s`, `/%s action:%s`, `/%s action:%s`, `/%s action:%s task_name:<name>`, `/%s action:%s task_name:<name>`, `/%s action:%s task_name:<name>`, or `/%s action:%s`.",
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
		commandName,
		actionTaskResetContext,
	)
}
