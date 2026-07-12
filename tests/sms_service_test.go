package tests

import (
	"context"
	"errors"
	"testing"

	app "sms_gateway/internal/application/sms"
	"sms_gateway/internal/domain/channel"
	domain "sms_gateway/internal/domain/sms"
	"sms_gateway/internal/domain/wallet"
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
	value  *wallet.Wallet
	err    error
	called bool
}

func (s *walletStub) DeductAndRecord(context.Context, int64, wallet.Money, string, string) (*wallet.Wallet, error) {
	s.called = true
	return s.value, s.err
}

type senderStub struct {
	err    error
	called bool
}

func (s *senderStub) Send(context.Context, domain.Destination, domain.LineType, string, string) error {
	s.called = true
	return s.err
}

func TestExecuteRejectsInvalidCommandBeforeDependencies(t *testing.T) {
	w := &walletStub{}
	sender := &senderStub{}
	svc := app.NewService(channelStub{}, w, costStub{1}, sender)
	_, err := svc.Execute(context.Background(), app.SendSMSCommand{})
	if err == nil || w.called || sender.called {
		t.Fatalf("err=%v wallet=%v sender=%v", err, w.called, sender.called)
	}
}

func TestExecuteChargesAndSends(t *testing.T) {
	w := &walletStub{value: &wallet.Wallet{Balance: wallet.MustMoney(8.5)}}
	sender := &senderStub{}
	svc := app.NewService(channelStub{value: &channel.Channel{WalletID: 1, IsActive: true}}, w, costStub{1.5}, sender)
	result, err := svc.Execute(context.Background(), app.SendSMSCommand{Line: domain.LineTypeNormal, Dest: "98912", ChannelName: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !w.called || !sender.called || result.Cost != 1.5 || result.RemainingBal != 8.5 {
		t.Fatalf("result=%+v", result)
	}
}

func TestExecuteReturnsSenderFailure(t *testing.T) {
	w := &walletStub{value: &wallet.Wallet{Balance: wallet.MustMoney(8.5)}}
	sender := &senderStub{err: errors.New("down")}
	svc := app.NewService(channelStub{value: &channel.Channel{WalletID: 1, IsActive: true}}, w, costStub{1.5}, sender)
	_, err := svc.Execute(context.Background(), app.SendSMSCommand{Line: domain.LineTypeNormal, Dest: "98912", ChannelName: "main"})
	if err == nil {
		t.Fatal("expected error")
	}
}
