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

type providerStateStore interface {
	next(context.Context) (uint64, error)
	acquire(context.Context, string) (uint64, bool, circuitState, bool, error)
	complete(context.Context, string, uint64, bool) (circuitState, bool, error)
	close() error
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

type memoryProviderState struct {
	nextIndex atomic.Uint64
	breakers  map[string]*circuitBreaker
}

func newMemoryProviderState(providers []Provider, failureThreshold int, cooldown time.Duration) *memoryProviderState {
	breakers := make(map[string]*circuitBreaker, len(providers))
	for _, provider := range providers {
		breakers[provider.Name()] = newCircuitBreaker(failureThreshold, cooldown)
	}
	return &memoryProviderState{breakers: breakers}
}

func (s *memoryProviderState) next(context.Context) (uint64, error) {
	return s.nextIndex.Add(1) - 1, nil
}

func (s *memoryProviderState) acquire(_ context.Context, provider string) (uint64, bool, circuitState, bool, error) {
	generation, allowed, state, changed := s.breakers[provider].acquire(time.Now())
	return generation, allowed, state, changed, nil
}

func (s *memoryProviderState) complete(_ context.Context, provider string, generation uint64, success bool) (circuitState, bool, error) {
	state, changed := s.breakers[provider].complete(generation, success, time.Now())
	return state, changed, nil
}

func (s *memoryProviderState) close() error { return nil }

// RoundRobinSender distributes calls across healthy provider circuits. Open
// circuits are skipped; an attempted provider failure is returned immediately
// so callers do not risk duplicate delivery by retrying a different provider.
type RoundRobinSender struct {
	providers []Provider
	state     providerStateStore
	logger    *slog.Logger
}

// NewRoundRobinSender creates an in-process pool. Production workers use the
// Redis constructor so selection and circuit state are coordinated across all
// replicas; this constructor remains useful for isolated tests and adapters.
func NewRoundRobinSender(providers []Provider, failureThreshold int, cooldown time.Duration, logger *slog.Logger) (*RoundRobinSender, error) {
	providers, logger, err := validatePool(providers, failureThreshold, cooldown, logger)
	if err != nil {
		return nil, err
	}
	return newRoundRobinSender(providers, newMemoryProviderState(providers, failureThreshold, cooldown), logger), nil
}

func newRoundRobinSender(providers []Provider, state providerStateStore, logger *slog.Logger) *RoundRobinSender {
	return &RoundRobinSender{providers: providers, state: state, logger: logger}
}

func validatePool(providers []Provider, failureThreshold int, cooldown time.Duration, logger *slog.Logger) ([]Provider, *slog.Logger, error) {
	if len(providers) == 0 {
		return nil, nil, fmt.Errorf("at least one SMS provider is required")
	}
	if failureThreshold < 1 {
		return nil, nil, fmt.Errorf("circuit breaker failure threshold must be positive")
	}
	if cooldown <= 0 {
		return nil, nil, fmt.Errorf("circuit breaker cooldown must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}

	seen := make(map[string]struct{}, len(providers))
	validated := make([]Provider, len(providers))
	copy(validated, providers)
	for _, provider := range validated {
		if provider == nil || provider.Name() == "" {
			return nil, nil, fmt.Errorf("SMS provider and provider name are required")
		}
		if _, exists := seen[provider.Name()]; exists {
			return nil, nil, fmt.Errorf("duplicate SMS provider name %q", provider.Name())
		}
		seen[provider.Name()] = struct{}{}
	}
	return validated, logger, nil
}

func defaultMockProviders(logger *slog.Logger) ([]Provider, error) {
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
	return providers, nil
}

func NewDefaultMockRoundRobinSender(logger *slog.Logger, failureThreshold int, cooldown time.Duration) (*RoundRobinSender, error) {
	providers, err := defaultMockProviders(logger)
	if err != nil {
		return nil, err
	}
	return NewRoundRobinSender(providers, failureThreshold, cooldown, logger)
}

func (s *RoundRobinSender) Close() error { return s.state.close() }

func (s *RoundRobinSender) Send(ctx context.Context, event app.DeliveryEvent) error {
	start, err := s.state.next(ctx)
	if err != nil {
		return fmt.Errorf("select SMS provider: %w", err)
	}
	for offset := range s.providers {
		index := (start + uint64(offset)) % uint64(len(s.providers))
		provider := s.providers[int(index)]
		generation, allowed, state, changed, err := s.state.acquire(ctx, provider.Name())
		if err != nil {
			return fmt.Errorf("read provider %s circuit: %w", provider.Name(), err)
		}
		if changed {
			s.logger.Info("SMS provider circuit state changed", "provider", provider.Name(), "state", state.String())
		}
		if !allowed {
			continue
		}

		sendErr := provider.Send(ctx, event)
		completionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		state, changed, completeErr := s.state.complete(completionCtx, provider.Name(), generation, sendErr == nil)
		cancel()
		if completeErr != nil {
			s.logger.Error("update SMS provider circuit", "provider", provider.Name(), "error", completeErr)
		} else if changed {
			s.logger.Warn("SMS provider circuit state changed", "provider", provider.Name(), "state", state.String())
		}
		if sendErr != nil {
			return fmt.Errorf("send with provider %s: %w", provider.Name(), sendErr)
		}
		return nil
	}
	return ErrAllProvidersUnavailable
}
