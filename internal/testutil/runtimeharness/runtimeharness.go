package runtimeharness

import (
	"fmt"
	"testing"
)

type Attachment struct {
	Filename    string
	ContentType string
	URL         string
}

type MessageEvent struct {
	UserID      string
	ChannelID   string
	MessageID   string
	Content     string
	Mentioned   bool
	Attachments []Attachment
}

type CommandIntent struct {
	CommandName string
	UserID      string
	Action      string
	TaskName    string
	TaskID      string
}

type DeliveryKind string

const (
	DeliveryKindChannelMessage      DeliveryKind = "channel_message"
	DeliveryKindChannelEdit         DeliveryKind = "channel_edit"
	DeliveryKindChannelDelete       DeliveryKind = "channel_delete"
	DeliveryKindInteractionResponse DeliveryKind = "interaction_response"
	DeliveryKindInteractionEdit     DeliveryKind = "interaction_edit"
	DeliveryKindInteractionFollowup DeliveryKind = "interaction_followup"
)

type Delivery struct {
	Kind      DeliveryKind
	ChannelID string
	MessageID string
	ReplyToID string
	Text      string
	Ephemeral bool
}

type Expectation struct {
	Kind      DeliveryKind
	ChannelID string
	MessageID string
	ReplyToID string
	Text      string
	Ephemeral *bool
}

func Bool(value bool) *bool {
	return &value
}

func RequireDeliveries(t testing.TB, got []Delivery, want ...Expectation) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("delivery count = %d, want %d\n got: %#v", len(got), len(want), got)
	}

	for index, expected := range want {
		actual := got[index]
		if actual.Kind != expected.Kind {
			t.Fatalf("delivery[%d] kind = %q, want %q", index, actual.Kind, expected.Kind)
		}

		if expected.ChannelID != "" && actual.ChannelID != expected.ChannelID {
			t.Fatalf("delivery[%d] channel id = %q, want %q", index, actual.ChannelID, expected.ChannelID)
		}

		if expected.MessageID != "" && actual.MessageID != expected.MessageID {
			t.Fatalf("delivery[%d] message id = %q, want %q", index, actual.MessageID, expected.MessageID)
		}

		if expected.ReplyToID != "" && actual.ReplyToID != expected.ReplyToID {
			t.Fatalf("delivery[%d] reply to id = %q, want %q", index, actual.ReplyToID, expected.ReplyToID)
		}

		if expected.Text != "" && actual.Text != expected.Text {
			t.Fatalf("delivery[%d] text = %q, want %q", index, actual.Text, expected.Text)
		}

		if expected.Ephemeral != nil && actual.Ephemeral != *expected.Ephemeral {
			t.Fatalf(
				"delivery[%d] ephemeral = %v, want %v",
				index,
				actual.Ephemeral,
				*expected.Ephemeral,
			)
		}
	}
}

func (d Delivery) String() string {
	return fmt.Sprintf(
		"Delivery{Kind:%q ChannelID:%q MessageID:%q ReplyToID:%q Text:%q Ephemeral:%v}",
		d.Kind,
		d.ChannelID,
		d.MessageID,
		d.ReplyToID,
		d.Text,
		d.Ephemeral,
	)
}
