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

type acceptorStub struct {
	event  app.DeliveryEvent
	called bool
	result *wallet.Wallet
	err    error
}

type isolatingAcceptor struct {
	entered chan struct{}
	release chan struct{}
}

type reservationStub struct {
	reserved  bool
	committed bool
	refunded  bool
}

func (r *reservationStub) Reserve(context.Context, int64, int64, string) (int64, error) {
	r.reserved = true
	return 76500, nil
}
func (r *reservationStub) IsReserved(context.Context, string) (bool, error) { return r.reserved, nil }
func (r *reservationStub) Commit(context.Context, string) error             { r.committed = true; return nil }
func (r *reservationStub) Refund(context.Context, int64, string) error      { r.refunded = true; return nil }

type enqueueStub struct {
	called bool
	err    error
}

func (e *enqueueStub) Enqueue(context.Context, app.DeliveryEvent, string) error {
	e.called = true
	return e.err
}

func (a *isolatingAcceptor) DeductAndEnqueue(ctx context.Context, _ int64, _ wallet.Money, event app.DeliveryEvent, _ string) (*wallet.Wallet, error) {
	if event.Line == domain.LineTypeExpress {
		close(a.entered)
		select {
		case <-a.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &wallet.Wallet{Balance: wallet.MustMoney(9)}, nil
}

func (s *acceptorStub) DeductAndEnqueue(_ context.Context, _ int64, _ wallet.Money, event app.DeliveryEvent, _ string) (*wallet.Wallet, error) {
	s.called = true
	s.event = event
	return s.result, s.err
}

func TestExecuteCreatesExpressBrokerEvent(t *testing.T) {
	a := &acceptorStub{result: &wallet.Wallet{Balance: wallet.MustMoney(8.5)}}
	svc := app.NewService(activeChannel(), a, costStub{1.5})
	result, err := svc.Execute(context.Background(), command(domain.LineTypeExpress))
	if err != nil {
		t.Fatal(err)
	}
	if !a.called || a.event.Line != domain.LineTypeExpress || a.event.MessageID != result.MessageID || a.event.CostUnits != 15000 {
		t.Fatalf("event=%+v result=%+v", a.event, result)
	}
	if got := a.event.DeadlineAt.Sub(a.event.CreatedAt); got != 5*time.Second {
		t.Fatalf("express SLA deadline = %s", got)
	}
}

func TestExecuteCreatesNormalBrokerEvent(t *testing.T) {
	a := &acceptorStub{result: &wallet.Wallet{Balance: wallet.MustMoney(9)}}
	svc := app.NewService(activeChannel(), a, costStub{1})
	if _, err := svc.Execute(context.Background(), command(domain.LineTypeNormal)); err != nil {
		t.Fatal(err)
	}
	if a.event.Line != domain.LineTypeNormal {
		t.Fatalf("line=%q", a.event.Line)
	}
	if !a.event.DeadlineAt.IsZero() {
		t.Fatalf("normal event must not have express deadline: %s", a.event.DeadlineAt)
	}
}

func TestExecuteUsesConfiguredExpressSLA(t *testing.T) {
	a := &acceptorStub{result: &wallet.Wallet{Balance: wallet.MustMoney(9)}}
	svc := app.NewServiceWithExpressSLA(activeChannel(), a, costStub{1}, 750*time.Millisecond)
	if _, err := svc.Execute(context.Background(), command(domain.LineTypeExpress)); err != nil {
		t.Fatal(err)
	}
	if got := a.event.DeadlineAt.Sub(a.event.CreatedAt); got != 750*time.Millisecond {
		t.Fatalf("deadline = %s", got)
	}
}

func TestReservationHappensBeforeEnqueue(t *testing.T) {
	reservation := &reservationStub{}
	enqueuer := &enqueueStub{}
	svc := app.NewServiceWithReservation(activeChannel(), enqueuer, costStub{1}, reservation, time.Second, 1, 1)
	result, err := svc.Execute(context.Background(), command(domain.LineTypeNormal))
	if err != nil {
		t.Fatal(err)
	}
	if !reservation.reserved || !enqueuer.called || result.RemainingBal != 7.65 {
		t.Fatalf("reservation=%+v enqueue=%+v result=%+v", reservation, enqueuer, result)
	}
}

func TestFailedEnqueueRefundsReservation(t *testing.T) {
	reservation := &reservationStub{}
	enqueuer := &enqueueStub{err: errors.New("outbox unavailable")}
	svc := app.NewServiceWithReservation(activeChannel(), enqueuer, costStub{1}, reservation, time.Second, 1, 1)
	if _, err := svc.Execute(context.Background(), command(domain.LineTypeNormal)); err == nil || !reservation.refunded {
		t.Fatalf("err=%v reservation=%+v", err, reservation)
	}
}

func TestAdmissionCapacityIsIsolatedPerLine(t *testing.T) {
	a := &isolatingAcceptor{entered: make(chan struct{}), release: make(chan struct{})}
	svc := app.NewServiceWithPolicy(activeChannel(), a, costStub{1}, time.Second, 1, 1)
	done := make(chan error, 1)
	go func() {
		_, err := svc.Execute(context.Background(), command(domain.LineTypeExpress))
		done <- err
	}()
	<-a.entered

	if _, err := svc.Execute(context.Background(), command(domain.LineTypeExpress)); !errors.Is(err, app.ErrLineOverloaded) {
		t.Fatalf("second express request error = %v", err)
	}
	if _, err := svc.Execute(context.Background(), command(domain.LineTypeNormal)); err != nil {
		t.Fatalf("normal request was starved by express load: %v", err)
	}
	close(a.release)
	if err := <-done; err != nil {
		t.Fatalf("first express request: %v", err)
	}
}

func TestExecuteRejectsInvalidBeforeOutbox(t *testing.T) {
	a := &acceptorStub{}
	svc := app.NewService(channelStub{}, a, costStub{1})
	if _, err := svc.Execute(context.Background(), app.SendSMSCommand{}); err == nil || a.called {
		t.Fatalf("err=%v called=%v", err, a.called)
	}
}

func TestSendSMSReturnsAccepted(t *testing.T) {
	a := &acceptorStub{result: &wallet.Wallet{Balance: wallet.MustMoney(9)}}
	svc := app.NewService(activeChannel(), a, costStub{1})
	r := router.Setup(handler.NewSMSHandler(svc, nil), "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms", bytes.NewBufferString(`{"line":"express","dest":"98912","channel":"main","message":"test message"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func activeChannel() channelStub {
	return channelStub{value: &channel.Channel{WalletID: 1, IsActive: true}}
}
func command(line domain.LineType) app.SendSMSCommand {
	return app.SendSMSCommand{Line: line, Dest: "98912", ChannelName: "main", Message: "test message"}
}
