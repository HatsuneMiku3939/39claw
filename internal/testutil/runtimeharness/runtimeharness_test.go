package runtimeharness

import "testing"

func TestRequireDeliveriesMatchesExpectedSequence(t *testing.T) {
	t.Parallel()

	got := []Delivery{
		{
			Kind:      DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			ReplyToID: "message-1",
			Text:      "hello",
		},
		{
			Kind:      DeliveryKindInteractionResponse,
			Text:      "done",
			Ephemeral: true,
		},
	}

	RequireDeliveries(t, got,
		Expectation{
			Kind:      DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			ReplyToID: "message-1",
			Text:      "hello",
		},
		Expectation{
			Kind:      DeliveryKindInteractionResponse,
			Text:      "done",
			Ephemeral: Bool(true),
		},
	)
}
