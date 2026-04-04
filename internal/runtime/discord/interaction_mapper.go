package discord

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	commandHelp = "help"
	commandTask = "task"

	taskActionCurrent = "current"
	taskActionList    = "list"
	taskActionNew     = "new"
	taskActionSwitch  = "switch"
	taskActionClose   = "close"
)

type commandRequest struct {
	Name   string
	UserID string
	Task   taskCommandRequest
}

type taskCommandRequest struct {
	Action string
	Name   string
	ID     string
}

func mapInteractionCommand(event *discordgo.InteractionCreate) (commandRequest, bool) {
	if event == nil || event.Interaction == nil || event.Type != discordgo.InteractionApplicationCommand {
		return commandRequest{}, false
	}

	data := event.ApplicationCommandData()
	request := commandRequest{
		Name:   data.Name,
		UserID: interactionUserID(event.Interaction),
	}

	if request.UserID == "" {
		return commandRequest{}, false
	}

	if data.Name != commandTask {
		return request, true
	}

	request.Task = mapTaskCommand(data.Options)
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

func mapTaskCommand(options []*discordgo.ApplicationCommandInteractionDataOption) taskCommandRequest {
	if len(options) == 0 {
		return taskCommandRequest{Action: taskActionCurrent}
	}

	subcommand := options[0]
	request := taskCommandRequest{
		Action: strings.TrimSpace(subcommand.Name),
	}

	switch request.Action {
	case taskActionNew:
		request.Name = optionStringValue(subcommand.Options, "name")
	case taskActionSwitch, taskActionClose:
		request.ID = optionStringValue(subcommand.Options, "id")
	}

	return request
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
