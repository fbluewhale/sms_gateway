package tests

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	app "sms_gateway/internal/application/sms"
	smsInfra "sms_gateway/internal/infrastructure/sms"
)

type recordingProvider struct {
	name      string
	mu        sync.Mutex
	calls     int
	responses []error
	order     *[]string
	orderMu   *sync.Mutex
}

type blockingProbeProvider struct {
	mu           sync.Mutex
	calls        int
	probeStarted chan struct{}
	releaseProbe chan struct{}
}

func (p *blockingProbeProvider) Name() string { return "blocking" }

func (p *blockingProbeProvider) Send(_ context.Context, _ app.DeliveryEvent) error {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if call == 1 {
		return errors.New("provider failure")
	}
	if call == 2 {
		close(p.probeStarted)
		<-p.releaseProbe
	}
	return nil
}

func (p *recordingProvider) Name() string { return p.name }

func (p *recordingProvider) Send(_ context.Context, _ app.DeliveryEvent) error {
	p.mu.Lock()
	index := p.calls
	p.calls++
	var err error
	if index < len(p.responses) {
		err = p.responses[index]
	}
	p.mu.Unlock()
	if p.order != nil {
		p.orderMu.Lock()
		*p.order = append(*p.order, p.name)
		p.orderMu.Unlock()
	}
	return err
}

func (p *recordingProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRoundRobinSenderDistributesRequestsInOrder(t *testing.T) {
	var order []string
	var orderMu sync.Mutex
	providers := []smsInfra.Provider{
		&recordingProvider{name: "alpha", order: &order, orderMu: &orderMu},
		&recordingProvider{name: "beta", order: &order, orderMu: &orderMu},
		&recordingProvider{name: "gamma", order: &order, orderMu: &orderMu},
	}
	sender, err := smsInfra.NewRoundRobinSender(providers, 2, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("NewRoundRobinSender() error = %v", err)
	}
	for range 6 {
		if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}
	want := []string{"alpha", "beta", "gamma", "alpha", "beta", "gamma"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestMockProviderRandomlySucceedsAndFails(t *testing.T) {
	provider, err := smsInfra.NewMockProvider("random", 0.5, 0, discardLogger())
	if err != nil {
		t.Fatalf("NewMockProvider() error = %v", err)
	}

	var successes, failures int
	for range 200 {
		if err := provider.Send(context.Background(), app.DeliveryEvent{}); err != nil {
			failures++
		} else {
			successes++
		}
	}
	if successes == 0 || failures == 0 {
		t.Fatalf("random results successes=%d failures=%d; want both outcomes", successes, failures)
	}
}

func TestRoundRobinSenderSkipsOnlyProvidersWithOpenCircuits(t *testing.T) {
	failed := errors.New("provider failure")
	alpha := &recordingProvider{name: "alpha", responses: []error{failed}}
	beta := &recordingProvider{name: "beta"}
	sender, err := smsInfra.NewRoundRobinSender([]smsInfra.Provider{alpha, beta}, 1, time.Hour, discardLogger())
	if err != nil {
		t.Fatalf("NewRoundRobinSender() error = %v", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, failed) {
		t.Fatalf("first Send() error = %v, want provider failure", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("third Send() error = %v", err)
	}
	if alpha.callCount() != 1 || beta.callCount() != 2 {
		t.Fatalf("calls alpha=%d beta=%d, want 1 and 2", alpha.callCount(), beta.callCount())
	}
}

func TestCircuitBreakerTransitionsThroughHalfOpen(t *testing.T) {
	failed := errors.New("provider failure")
	provider := &recordingProvider{name: "alpha", responses: []error{failed, nil, nil}}
	sender, err := smsInfra.NewRoundRobinSender([]smsInfra.Provider{provider}, 1, 10*time.Millisecond, discardLogger())
	if err != nil {
		t.Fatalf("NewRoundRobinSender() error = %v", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, failed) {
		t.Fatalf("first Send() error = %v, want provider failure", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, smsInfra.ErrAllProvidersUnavailable) {
		t.Fatalf("open-circuit Send() error = %v", err)
	}
	time.Sleep(15 * time.Millisecond)
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("half-open probe Send() error = %v", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("closed-circuit Send() error = %v", err)
	}
	if provider.callCount() != 3 {
		t.Fatalf("provider calls = %d, want 3", provider.callCount())
	}
}

func TestCircuitBreakerAllowsOnlyOneHalfOpenProbe(t *testing.T) {
	provider := &blockingProbeProvider{probeStarted: make(chan struct{}), releaseProbe: make(chan struct{})}
	sender, err := smsInfra.NewRoundRobinSender([]smsInfra.Provider{provider}, 1, 10*time.Millisecond, discardLogger())
	if err != nil {
		t.Fatalf("NewRoundRobinSender() error = %v", err)
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); err == nil {
		t.Fatal("first Send() error = nil, want provider failure")
	}
	time.Sleep(15 * time.Millisecond)

	probeResult := make(chan error, 1)
	go func() { probeResult <- sender.Send(context.Background(), app.DeliveryEvent{}) }()
	select {
	case <-provider.probeStarted:
	case <-time.After(time.Second):
		t.Fatal("half-open probe did not start")
	}
	if err := sender.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, smsInfra.ErrAllProvidersUnavailable) {
		t.Fatalf("concurrent half-open Send() error = %v", err)
	}
	close(provider.releaseProbe)
	if err := <-probeResult; err != nil {
		t.Fatalf("half-open probe error = %v", err)
	}
}

func TestRoundRobinSenderIsSafeForConcurrentUse(t *testing.T) {
	providers := []*recordingProvider{{name: "alpha"}, {name: "beta"}, {name: "gamma"}}
	interfaces := make([]smsInfra.Provider, len(providers))
	for i := range providers {
		interfaces[i] = providers[i]
	}
	sender, err := smsInfra.NewRoundRobinSender(interfaces, 3, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("NewRoundRobinSender() error = %v", err)
	}
	var wg sync.WaitGroup
	for range 300 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
				t.Errorf("Send() error = %v", err)
			}
		}()
	}
	wg.Wait()
	for _, provider := range providers {
		if provider.callCount() != 100 {
			t.Fatalf("provider %s calls = %d, want 100", provider.name, provider.callCount())
		}
	}
}

func TestRedisProviderPoolSharesRoundRobinAcrossSenders(t *testing.T) {
	redisURL := redisTestURL(t)
	var order []string
	var orderMu sync.Mutex
	newProviders := func() []smsInfra.Provider {
		return []smsInfra.Provider{
			&recordingProvider{name: "alpha", order: &order, orderMu: &orderMu},
			&recordingProvider{name: "beta", order: &order, orderMu: &orderMu},
			&recordingProvider{name: "gamma", order: &order, orderMu: &orderMu},
		}
	}
	poolName := uniqueRedisPoolName(t)
	first, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, newProviders(), 2, time.Second, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("first NewRedisRoundRobinSender() error = %v", err)
	}
	defer first.Close()
	second, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, newProviders(), 2, time.Second, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("second NewRedisRoundRobinSender() error = %v", err)
	}
	defer second.Close()

	for _, sender := range []*smsInfra.RoundRobinSender{first, second, first, second, first, second} {
		if err := sender.Send(context.Background(), app.DeliveryEvent{}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}
	want := []string{"alpha", "beta", "gamma", "alpha", "beta", "gamma"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d; full order=%v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q; full order=%v", i, order[i], want[i], order)
		}
	}
}

func TestRedisCircuitIsSharedAcrossSenders(t *testing.T) {
	redisURL := redisTestURL(t)
	failed := errors.New("provider failure")
	firstAlpha := &recordingProvider{name: "alpha", responses: []error{failed}}
	firstBeta := &recordingProvider{name: "beta"}
	secondAlpha := &recordingProvider{name: "alpha"}
	secondBeta := &recordingProvider{name: "beta"}
	poolName := uniqueRedisPoolName(t)
	first, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, []smsInfra.Provider{firstAlpha, firstBeta}, 1, time.Hour, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("first NewRedisRoundRobinSender() error = %v", err)
	}
	defer first.Close()
	second, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, []smsInfra.Provider{secondAlpha, secondBeta}, 1, time.Hour, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("second NewRedisRoundRobinSender() error = %v", err)
	}
	defer second.Close()

	if err := first.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, failed) {
		t.Fatalf("first Send() error = %v, want provider failure", err)
	}
	if err := second.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	if err := second.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("third Send() error = %v", err)
	}
	if secondAlpha.callCount() != 0 || secondBeta.callCount() != 2 {
		t.Fatalf("second sender calls alpha=%d beta=%d, want 0 and 2", secondAlpha.callCount(), secondBeta.callCount())
	}
}

func TestRedisCircuitAllowsOneHalfOpenProbeAcrossSenders(t *testing.T) {
	redisURL := redisTestURL(t)
	firstProvider := &blockingProbeProvider{probeStarted: make(chan struct{}), releaseProbe: make(chan struct{})}
	secondProvider := &recordingProvider{name: "blocking"}
	poolName := uniqueRedisPoolName(t)
	halfOpenLease := 40 * time.Millisecond
	first, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, []smsInfra.Provider{firstProvider}, 1, 20*time.Millisecond, halfOpenLease, discardLogger())
	if err != nil {
		t.Fatalf("first NewRedisRoundRobinSender() error = %v", err)
	}
	defer first.Close()
	second, err := smsInfra.NewRedisRoundRobinSender(context.Background(), redisURL, poolName, []smsInfra.Provider{secondProvider}, 1, 20*time.Millisecond, halfOpenLease, discardLogger())
	if err != nil {
		t.Fatalf("second NewRedisRoundRobinSender() error = %v", err)
	}
	defer second.Close()

	if err := first.Send(context.Background(), app.DeliveryEvent{}); err == nil {
		t.Fatal("first Send() error = nil, want provider failure")
	}
	if err := second.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, smsInfra.ErrAllProvidersUnavailable) {
		t.Fatalf("open-circuit Send() error = %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	probeResult := make(chan error, 1)
	go func() { probeResult <- first.Send(context.Background(), app.DeliveryEvent{}) }()
	select {
	case <-firstProvider.probeStarted:
	case <-time.After(time.Second):
		t.Fatal("half-open probe did not start")
	}
	if err := second.Send(context.Background(), app.DeliveryEvent{}); !errors.Is(err, smsInfra.ErrAllProvidersUnavailable) {
		t.Fatalf("concurrent half-open Send() error = %v", err)
	}
	time.Sleep(halfOpenLease + 10*time.Millisecond)
	if err := second.Send(context.Background(), app.DeliveryEvent{}); err != nil {
		t.Fatalf("replacement half-open probe error = %v", err)
	}
	if secondProvider.callCount() != 1 {
		t.Fatalf("replacement provider calls = %d, want 1", secondProvider.callCount())
	}
	close(firstProvider.releaseProbe)
	if err := <-probeResult; err != nil {
		t.Fatalf("half-open probe error = %v", err)
	}
}

func redisTestURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}
	return url
}

func uniqueRedisPoolName(t *testing.T) string {
	t.Helper()
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}
