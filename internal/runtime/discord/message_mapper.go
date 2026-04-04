package discord

import (
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/bwmarrin/discordgo"
)

func mapMessageCreate(selfUserID string, event *discordgo.MessageCreate) (app.MessageRequest, bool) {
	if event == nil || event.Message == nil || event.Author == nil {
		return app.MessageRequest{}, false
	}

	if event.Author.Bot || event.Author.ID == selfUserID {
		return app.MessageRequest{}, false
	}

	if !mentionsUser(event.Mentions, selfUserID) {
		return app.MessageRequest{}, false
	}

	content := strings.TrimSpace(stripSelfMentions(event.Content, selfUserID))

	receivedAt := event.Timestamp
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	return app.MessageRequest{
		UserID:     event.Author.ID,
		ChannelID:  event.ChannelID,
		MessageID:  event.ID,
		Content:    content,
		Mentioned:  true,
		ReceivedAt: receivedAt,
	}, true
}

func mentionsUser(users []*discordgo.User, userID string) bool {
	for _, user := range users {
		if user != nil && user.ID == userID {
			return true
		}
	}

	return false
}

func stripSelfMentions(content string, selfUserID string) string {
	replacer := strings.NewReplacer(
		"<@"+selfUserID+">", " ",
		"<@!"+selfUserID+">", " ",
	)
	return replacer.Replace(content)
}
