package discord

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	optionAction   = "action"
	optionTaskName = "task_name"
	optionTaskID   = "task_id"

	actionHelp             = "help"
	actionClear            = "clear"
	actionTaskCurrent      = "task-current"
	actionTaskList         = "task-list"
	actionTaskNew          = "task-new"
	actionTaskSwitch       = "task-switch"
	actionTaskClose        = "task-close"
	actionTaskResetContext = "task-reset-context"
)

type commandRequest struct {
	Name     string
	Action   string
	UserID   string
	TaskName string
	TaskID   string
}

func mapInteractionCommand(event *discordgo.InteractionCreate) (commandRequest, bool) {
	if event == nil || event.Interaction == nil || event.Type != discordgo.InteractionApplicationCommand {
		return commandRequest{}, false
	}

	data := event.ApplicationCommandData()
	request := commandRequest{
		Name:     data.Name,
		Action:   optionStringValue(data.Options, optionAction),
		UserID:   interactionUserID(event.Interaction),
		TaskName: optionStringValue(data.Options, optionTaskName),
		TaskID:   optionStringValue(data.Options, optionTaskID),
	}

	if request.UserID == "" {
		return commandRequest{}, false
	}

	return request, true
}

func interactionUserID(interaction *discordgo.Interaction) string {
	if interaction == nil {
		return ""
	}

	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}

	if interaction.User != nil {
		return interaction.User.ID
	}

	return ""
}

func optionStringValue(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}

		if option.Type != discordgo.ApplicationCommandOptionString {
			return ""
		}

		return strings.TrimSpace(option.StringValue())
	}

	return ""
}
