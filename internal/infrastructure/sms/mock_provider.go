package sms

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	app "sms_gateway/internal/application/sms"
)

// MockProvider simulates an SMS provider with provider-specific latency and a
// random failure rate. It is intended for local development and tests only.
type MockProvider struct {
	name        string
	failureRate float64
	latency     time.Duration
	logger      *slog.Logger
}

func NewMockProvider(name string, failureRate float64, latency time.Duration, logger *slog.Logger) (*MockProvider, error) {
	if name == "" {
		return nil, fmt.Errorf("mock provider name is required")
	}
	if failureRate < 0 || failureRate > 1 {
		return nil, fmt.Errorf("mock provider %q failure rate must be between 0 and 1", name)
	}
	if latency < 0 {
		return nil, fmt.Errorf("mock provider %q latency cannot be negative", name)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MockProvider{name: name, failureRate: failureRate, latency: latency, logger: logger}, nil
}

func (p *MockProvider) Name() string { return p.name }

func (p *MockProvider) Send(ctx context.Context, event app.DeliveryEvent) error {
	if p.latency > 0 {
		timer := time.NewTimer(p.latency)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return fmt.Errorf("provider %s send canceled: %w", p.name, ctx.Err())
		case <-timer.C:
		}
	}

	if rand.Float64() < p.failureRate {
		p.logger.Warn("mock SMS provider failed", "provider", p.name, "message_id", event.MessageID)
		return fmt.Errorf("provider %s rejected SMS", p.name)
	}
	p.logger.Info("mock SMS sent", "provider", p.name, "message_id", event.MessageID, "line", event.Line, "channel", event.ChannelName, "message_length", len([]rune(event.Message)))
	return nil
}
