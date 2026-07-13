package sms

import (
	"context"
	"log/slog"

	app "sms_gateway/internal/application/sms"
)

type MockSender struct{ logger *slog.Logger }

func NewMockSender(logger *slog.Logger) *MockSender { return &MockSender{logger: logger} }

func (s *MockSender) Send(_ context.Context, event app.DeliveryEvent) error {
	s.logger.Info("mock SMS sent", "message_id", event.MessageID, "line", event.Line, "channel", event.ChannelName)
	return nil
}
