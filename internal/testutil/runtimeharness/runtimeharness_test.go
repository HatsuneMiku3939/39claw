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

func TestRequireDeliveriesChecksMessageIDAndFalseEphemeral(t *testing.T) {
	t.Parallel()

	got := []Delivery{
		{
			Kind:      DeliveryKindChannelEdit,
			ChannelID: "channel-2",
			MessageID: "message-2",
			Text:      "updated",
			Ephemeral: false,
		},
	}

	RequireDeliveries(t, got,
		Expectation{
			Kind:      DeliveryKindChannelEdit,
			ChannelID: "channel-2",
			MessageID: "message-2",
			Text:      "updated",
			Ephemeral: Bool(false),
		},
	)
}

func TestDeliveryString(t *testing.T) {
	t.Parallel()

	delivery := Delivery{
		Kind:      DeliveryKindInteractionFollowup,
		ChannelID: "channel-3",
		MessageID: "message-3",
		ReplyToID: "reply-3",
		Text:      "done",
		Ephemeral: true,
	}

	got := delivery.String()
	want := "Delivery{Kind:\"interaction_followup\" ChannelID:\"channel-3\" MessageID:\"message-3\" ReplyToID:\"reply-3\" Text:\"done\" Ephemeral:true}"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
