package tests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms", bytes.NewBufferString(`{"line":"express","dest":"98912","channel":"main"}`))
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
	return app.SendSMSCommand{Line: line, Dest: "98912", ChannelName: "main"}
}
