package discord

import (
	"fmt"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/bwmarrin/discordgo"
)

func presentMessage(discordSession session, channelID string, response app.MessageResponse) (string, error) {
	if response.Ignore {
		return "", nil
	}

	chunks := chunkText(response.Text)
	if len(chunks) == 0 {
		return "", nil
	}

	primaryMessageID := ""

	for index, chunk := range chunks {
		payload := &discordgo.MessageSend{
			Content:         chunk,
			AllowedMentions: disallowMentions(),
		}
		if index == 0 && response.ReplyToID != "" {
			payload.Reference = replyReference(channelID, response.ReplyToID)
		}

		message, err := discordSession.ChannelMessageSendComplex(channelID, payload)
		if err != nil {
			return "", fmt.Errorf("send channel message: %w", err)
		}

		if index == 0 && message != nil {
			primaryMessageID = message.ID
		}
	}

	return primaryMessageID, nil
}

func presentInteraction(discordSession session, interaction *discordgo.Interaction, response app.MessageResponse) error {
	if response.Ignore {
		return nil
	}

	chunks := chunkText(response.Text)
	if len(chunks) == 0 {
		return nil
	}

	flags := discordgo.MessageFlags(0)
	if response.Ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	if len(chunks) == 1 {
		if err := discordSession.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:         chunks[0],
				Flags:           flags,
				AllowedMentions: disallowMentions(),
			},
		}); err != nil {
			return fmt.Errorf("respond to interaction: %w", err)
		}

		return nil
	}

	if err := discordSession.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: flags,
		},
	}); err != nil {
		return fmt.Errorf("defer interaction response: %w", err)
	}

	content := chunks[0]
	if _, err := discordSession.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: disallowMentions(),
	}); err != nil {
		return fmt.Errorf("edit deferred interaction response: %w", err)
	}

	for _, chunk := range chunks[1:] {
		if _, err := discordSession.FollowupMessageCreate(interaction, true, &discordgo.WebhookParams{
			Content:         chunk,
			Flags:           flags,
			AllowedMentions: disallowMentions(),
		}); err != nil {
			return fmt.Errorf("create interaction followup: %w", err)
		}
	}

	return nil
}

func disallowMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{
		Parse:       []discordgo.AllowedMentionType{},
		RepliedUser: false,
		Roles:       nil,
		Users:       nil,
	}
}

func replyReference(channelID string, messageID string) *discordgo.MessageReference {
	failIfNotExists := false

	return &discordgo.MessageReference{
		ChannelID:       channelID,
		MessageID:       messageID,
		FailIfNotExists: &failIfNotExists,
	}
}
