package sms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	app "sms_gateway/internal/application/sms"
)

var ErrAllProvidersUnavailable = errors.New("all SMS providers are unavailable")

type Provider interface {
	Name() string
	Send(context.Context, app.DeliveryEvent) error
}

type circuitState uint8

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

func (s circuitState) String() string {
	switch s {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

type circuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	failures         int
	failureThreshold int
	cooldown         time.Duration
	openedAt         time.Time
	halfOpenInFlight bool
	generation       uint64
}

func newCircuitBreaker(failureThreshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{failureThreshold: failureThreshold, cooldown: cooldown}
}

func (b *circuitBreaker) acquire(now time.Time) (uint64, bool, circuitState, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	previous := b.state
	if b.state == stateOpen {
		if now.Sub(b.openedAt) < b.cooldown {
			return 0, false, b.state, false
		}
		b.state = stateHalfOpen
	}
	if b.state == stateHalfOpen {
		if b.halfOpenInFlight {
			return 0, false, b.state, previous != b.state
		}
		b.halfOpenInFlight = true
	}
	return b.generation, true, b.state, previous != b.state
}

func (b *circuitBreaker) complete(generation uint64, success bool, now time.Time) (circuitState, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if generation != b.generation {
		return b.state, false
	}
	previous := b.state
	switch b.state {
	case stateHalfOpen:
		b.halfOpenInFlight = false
		b.generation++
		if success {
			b.state = stateClosed
			b.failures = 0
		} else {
			b.state = stateOpen
			b.openedAt = now
		}
	case stateClosed:
		if success {
			b.failures = 0
		} else {
			b.failures++
			if b.failures >= b.failureThreshold {
				b.state = stateOpen
				b.openedAt = now
				b.generation++
			}
		}
	}
	return b.state, previous != b.state
}

type protectedProvider struct {
	provider Provider
	breaker  *circuitBreaker
	logger   *slog.Logger
}

func (p *protectedProvider) trySend(ctx context.Context, event app.DeliveryEvent) (bool, error) {
	generation, allowed, state, changed := p.breaker.acquire(time.Now())
	if changed {
		p.logger.Info("SMS provider circuit state changed", "provider", p.provider.Name(), "state", state.String())
	}
	if !allowed {
		return false, nil
	}
	err := p.provider.Send(ctx, event)
	state, changed = p.breaker.complete(generation, err == nil, time.Now())
	if changed {
		p.logger.Warn("SMS provider circuit state changed", "provider", p.provider.Name(), "state", state.String())
	}
	if err != nil {
		return true, fmt.Errorf("send with provider %s: %w", p.provider.Name(), err)
	}
	return true, nil
}

// RoundRobinSender distributes calls across healthy provider circuits. Open
// circuits are skipped; an attempted provider failure is returned immediately
// so callers do not risk duplicate delivery by retrying a different provider.
type RoundRobinSender struct {
	providers []*protectedProvider
	next      atomic.Uint64
}

func NewRoundRobinSender(providers []Provider, failureThreshold int, cooldown time.Duration, logger *slog.Logger) (*RoundRobinSender, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one SMS provider is required")
	}
	if failureThreshold < 1 {
		return nil, fmt.Errorf("circuit breaker failure threshold must be positive")
	}
	if cooldown <= 0 {
		return nil, fmt.Errorf("circuit breaker cooldown must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}

	seen := make(map[string]struct{}, len(providers))
	protected := make([]*protectedProvider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil || provider.Name() == "" {
			return nil, fmt.Errorf("SMS provider and provider name are required")
		}
		if _, exists := seen[provider.Name()]; exists {
			return nil, fmt.Errorf("duplicate SMS provider name %q", provider.Name())
		}
		seen[provider.Name()] = struct{}{}
		protected = append(protected, &protectedProvider{provider: provider, breaker: newCircuitBreaker(failureThreshold, cooldown), logger: logger})
	}
	return &RoundRobinSender{providers: protected}, nil
}

func NewDefaultMockRoundRobinSender(logger *slog.Logger, failureThreshold int, cooldown time.Duration) (*RoundRobinSender, error) {
	specs := []struct {
		name        string
		failureRate float64
		latency     time.Duration
	}{
		{name: "mock-arya", failureRate: 0.10, latency: 20 * time.Millisecond},
		{name: "mock-baran", failureRate: 0.20, latency: 35 * time.Millisecond},
		{name: "mock-caspian", failureRate: 0.05, latency: 50 * time.Millisecond},
	}
	providers := make([]Provider, 0, len(specs))
	for _, spec := range specs {
		provider, err := NewMockProvider(spec.name, spec.failureRate, spec.latency, logger)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return NewRoundRobinSender(providers, failureThreshold, cooldown, logger)
}

func (s *RoundRobinSender) Send(ctx context.Context, event app.DeliveryEvent) error {
	start := s.next.Add(1) - 1
	for offset := range s.providers {
		index := (start + uint64(offset)) % uint64(len(s.providers))
		provider := s.providers[int(index)]
		attempted, err := provider.trySend(ctx, event)
		if attempted {
			return err
		}
	}
	return ErrAllProvidersUnavailable
}
