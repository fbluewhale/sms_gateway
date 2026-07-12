package sms

import (
	"context"
	"log/slog"

	domain "sms_gateway/internal/domain/sms"
)

type MockSender struct{ logger *slog.Logger }

func NewMockSender(logger *slog.Logger) *MockSender { return &MockSender{logger: logger} }

func (s *MockSender) Send(_ context.Context, _ domain.Destination, line domain.LineType, channel, messageID string) error {
	s.logger.Info("mock SMS sent", "message_id", messageID, "line", line, "channel", channel)
	return nil
}
