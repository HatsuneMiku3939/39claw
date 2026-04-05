package discord

import (
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type liveMessagePresenter struct {
	session   session
	channelID string
	replyToID string
	workdir   string

	mu         sync.Mutex
	messageIDs []string
	contents   []string
}

func newLiveMessagePresenter(
	discordSession session,
	channelID string,
	replyToID string,
	workdir string,
) *liveMessagePresenter {
	return &liveMessagePresenter{
		session:   discordSession,
		channelID: channelID,
		replyToID: replyToID,
		workdir:   workdir,
	}
}

func (p *liveMessagePresenter) Active() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.messageIDs) > 0
}

func (p *liveMessagePresenter) Update(text string) error {
	return p.syncText(text)
}

func (p *liveMessagePresenter) syncText(text string) error {
	text = formatDiscordResponseText(text, p.workdir)
	chunks := chunkText(text)
	if len(chunks) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.syncChunksLocked(chunks)
}

func (p *liveMessagePresenter) syncChunksLocked(chunks []string) error {
	limit := minInt(len(chunks), len(p.messageIDs))
	for index := 0; index < limit; index++ {
		if p.contents[index] == chunks[index] {
			continue
		}

		edit := discordgo.NewMessageEdit(p.channelID, p.messageIDs[index]).
			SetContent(chunks[index])
		edit.AllowedMentions = disallowMentions()

		if _, err := p.session.ChannelMessageEditComplex(edit); err != nil {
			return fmt.Errorf("edit channel message: %w", err)
		}

		p.contents[index] = chunks[index]
	}

	for index := len(p.messageIDs); index < len(chunks); index++ {
		payload := &discordgo.MessageSend{
			Content:         chunks[index],
			AllowedMentions: disallowMentions(),
		}
		if index == 0 && p.replyToID != "" {
			payload.Reference = replyReference(p.channelID, p.replyToID)
		}

		message, err := p.session.ChannelMessageSendComplex(p.channelID, payload)
		if err != nil {
			return fmt.Errorf("send channel message: %w", err)
		}

		messageID := ""
		if message != nil {
			messageID = message.ID
		}

		p.messageIDs = append(p.messageIDs, messageID)
		p.contents = append(p.contents, chunks[index])
	}

	for len(p.messageIDs) > len(chunks) {
		lastIndex := len(p.messageIDs) - 1
		messageID := p.messageIDs[lastIndex]
		if strings.TrimSpace(messageID) != "" {
			if err := p.session.ChannelMessageDelete(p.channelID, messageID); err != nil {
				return fmt.Errorf("delete channel message: %w", err)
			}
		}

		p.messageIDs = p.messageIDs[:lastIndex]
		p.contents = p.contents[:lastIndex]
	}

	return nil
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}

	return right
}
