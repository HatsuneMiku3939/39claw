package discord

import (
	"github.com/bwmarrin/discordgo"
)

type session interface {
	AddHandler(handler interface{}) func()
	Open() error
	Close() error
	ApplicationCommandBulkOverwrite(
		appID string,
		guildID string,
		commands []*discordgo.ApplicationCommand,
		options ...discordgo.RequestOption,
	) ([]*discordgo.ApplicationCommand, error)
	ChannelMessageSendComplex(
		channelID string,
		data *discordgo.MessageSend,
		options ...discordgo.RequestOption,
	) (*discordgo.Message, error)
	ChannelMessageEditComplex(
		data *discordgo.MessageEdit,
		options ...discordgo.RequestOption,
	) (*discordgo.Message, error)
	ChannelMessageDelete(
		channelID string,
		messageID string,
		options ...discordgo.RequestOption,
	) error
	InteractionRespond(
		interaction *discordgo.Interaction,
		resp *discordgo.InteractionResponse,
		options ...discordgo.RequestOption,
	) error
	InteractionResponseEdit(
		interaction *discordgo.Interaction,
		newresp *discordgo.WebhookEdit,
		options ...discordgo.RequestOption,
	) (*discordgo.Message, error)
	FollowupMessageCreate(
		interaction *discordgo.Interaction,
		wait bool,
		data *discordgo.WebhookParams,
		options ...discordgo.RequestOption,
	) (*discordgo.Message, error)
	SelfUserID() string
}

type sessionFactory func(token string) (session, error)

type liveSession struct {
	*discordgo.Session
}

func newLiveSession(token string) (session, error) {
	discordSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	discordSession.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent

	return &liveSession{Session: discordSession}, nil
}

func (s *liveSession) SelfUserID() string {
	if s.Session == nil || s.State == nil || s.State.User == nil {
		return ""
	}

	return s.State.User.ID
}
