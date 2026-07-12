package tests

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app "sms_gateway/internal/application/sms"
	"sms_gateway/internal/domain/channel"
	domain "sms_gateway/internal/domain/sms"
	"sms_gateway/internal/domain/wallet"
	"sms_gateway/internal/interfaces/http/handler"
	"sms_gateway/internal/interfaces/http/router"
)

type channelStub struct {
	value *channel.Channel
	err   error
}

func (s channelStub) GetByName(context.Context, string) (*channel.Channel, error) {
	return s.value, s.err
}

type costStub struct{ cost float64 }

func (s costStub) GetByLineType(context.Context, domain.LineType) (*domain.SMSCost, error) {
	return &domain.SMSCost{Cost: s.cost}, nil
}

type walletStub struct {
	value       *wallet.Wallet
	err         error
	called      bool
	creditCalls chan refundCall
}

func (s *walletStub) DeductAndRecord(context.Context, int64, wallet.Money, string, string) (*wallet.Wallet, error) {
	s.called = true
	return s.value, s.err
}

type refundCall struct {
	walletID int64
	amount   wallet.Money
	refID    string
	desc     string
}

func (s *walletStub) CreditAndRecord(_ context.Context, walletID int64, amount wallet.Money, refID, desc string) (*wallet.Wallet, error) {
	if s.creditCalls != nil {
		s.creditCalls <- refundCall{walletID: walletID, amount: amount, refID: refID, desc: desc}
	}
	return s.value, s.err
}

type senderStub struct {
	err   error
	calls chan domain.LineType
}

func (s *senderStub) Send(_ context.Context, _ domain.Destination, line domain.LineType, _, _ string) error {
	s.calls <- line
	return s.err
}

func TestExecuteRejectsInvalidCommandBeforeDependencies(t *testing.T) {
	w := &walletStub{}
	sender := &senderStub{calls: make(chan domain.LineType, 1)}
	svc := app.NewService(channelStub{}, w, costStub{cost: 1}, sender)
	t.Cleanup(func() { shutdownService(t, svc) })

	_, err := svc.Execute(context.Background(), app.SendSMSCommand{})
	if err == nil || w.called {
		t.Fatalf("err=%v wallet=%v", err, w.called)
	}
	select {
	case line := <-sender.calls:
		t.Fatalf("sender called for %q", line)
	default:
	}
}

func TestExecuteChargesAndQueuesWithoutWaitingForDelivery(t *testing.T) {
	release := make(chan struct{})
	sender := &blockingSender{normalStarted: make(chan struct{}, 1), releaseNormal: release, expressDelivered: make(chan struct{}, 1)}
	w := &walletStub{
		value:       &wallet.Wallet{Balance: wallet.MustMoney(8.5)},
		creditCalls: make(chan refundCall, 1),
	}
	svc := app.NewService(activeChannel(), w, costStub{cost: 1.5}, sender)

	result, err := svc.Execute(context.Background(), command(domain.LineTypeNormal))
	if err != nil {
		t.Fatal(err)
	}
	if !w.called || result.Cost != 1.5 || result.RemainingBal != 8.5 {
		t.Fatalf("result=%+v wallet_called=%v", result, w.called)
	}
	waitSignal(t, sender.normalStarted, "normal worker did not receive message")
	close(release)
	shutdownService(t, svc)
	select {
	case refund := <-w.creditCalls:
		t.Fatalf("successful SMS was refunded: %+v", refund)
	default:
	}
}

func TestExpressAndNormalUseIndependentChannels(t *testing.T) {
	sender := &blockingSender{
		normalStarted:    make(chan struct{}, 1),
		releaseNormal:    make(chan struct{}),
		expressDelivered: make(chan struct{}, 1),
	}
	w := &walletStub{value: &wallet.Wallet{Balance: wallet.MustMoney(8.5)}}
	svc := app.NewService(activeChannel(), w, costStub{cost: 1}, sender)

	if _, err := svc.Execute(context.Background(), command(domain.LineTypeNormal)); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, sender.normalStarted, "normal worker did not start")

	if _, err := svc.Execute(context.Background(), command(domain.LineTypeExpress)); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, sender.expressDelivered, "express message was blocked by normal channel")

	close(sender.releaseNormal)
	shutdownService(t, svc)
}

func TestSenderFailureDoesNotFailAcceptedRequest(t *testing.T) {
	sender := &senderStub{err: errors.New("down"), calls: make(chan domain.LineType, 1)}
	w := &walletStub{
		value:       &wallet.Wallet{Balance: wallet.MustMoney(10)},
		creditCalls: make(chan refundCall, 1),
	}
	svc := app.NewService(activeChannel(), w, costStub{cost: 1.5}, sender)

	if _, err := svc.Execute(context.Background(), command(domain.LineTypeNormal)); err != nil {
		t.Fatalf("Execute() error = %v; delivery happens asynchronously", err)
	}
	select {
	case line := <-sender.calls:
		if line != domain.LineTypeNormal {
			t.Fatalf("line = %q; want normal", line)
		}
	case <-time.After(time.Second):
		t.Fatal("sender was not called")
	}
	select {
	case refund := <-w.creditCalls:
		if refund.walletID != 1 || refund.amount.Units() != wallet.MustMoney(1.5).Units() {
			t.Fatalf("refund = %+v; want wallet 1 and amount 1.5", refund)
		}
		if refund.refID == "" || refund.desc == "" {
			t.Fatalf("refund must record message reference and description: %+v", refund)
		}
	case <-time.After(time.Second):
		t.Fatal("failed SMS was not refunded")
	}
	shutdownService(t, svc)
}

func TestSendSMSReturnsAccepted(t *testing.T) {
	sender := &senderStub{calls: make(chan domain.LineType, 1)}
	w := &walletStub{value: &wallet.Wallet{Balance: wallet.MustMoney(8.5)}}
	svc := app.NewService(activeChannel(), w, costStub{cost: 1}, sender)
	h := handler.NewSMSHandler(svc, nil)
	r := router.Setup(h, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms", bytes.NewBufferString(
		`{"line":"express","dest":"98912","channel":"main"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	r.ServeHTTP(response, req)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d; want %d; body=%s", response.Code, http.StatusAccepted, response.Body.String())
	}
	select {
	case line := <-sender.calls:
		if line != domain.LineTypeExpress {
			t.Fatalf("line = %q; want express", line)
		}
	case <-time.After(time.Second):
		t.Fatal("express sender was not called")
	}
	shutdownService(t, svc)
}

type blockingSender struct {
	normalStarted    chan struct{}
	releaseNormal    chan struct{}
	expressDelivered chan struct{}
}

func (s *blockingSender) Send(ctx context.Context, _ domain.Destination, line domain.LineType, _, _ string) error {
	switch line {
	case domain.LineTypeNormal:
		s.normalStarted <- struct{}{}
		select {
		case <-s.releaseNormal:
		case <-ctx.Done():
			return ctx.Err()
		}
	case domain.LineTypeExpress:
		s.expressDelivered <- struct{}{}
	}
	return nil
}

func activeChannel() channelStub {
	return channelStub{value: &channel.Channel{WalletID: 1, IsActive: true}}
}

func command(line domain.LineType) app.SendSMSCommand {
	return app.SendSMSCommand{Line: line, Dest: "98912", ChannelName: "main"}
}

func waitSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}

func shutdownService(t *testing.T, svc *app.Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
