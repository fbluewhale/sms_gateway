package sms

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"sms_gateway/internal/domain/channel"
	smsDomain "sms_gateway/internal/domain/sms"
	"sms_gateway/internal/domain/wallet"
)

type SendSMSCommand struct {
	Line        smsDomain.LineType
	Dest        smsDomain.Destination
	ChannelName string
}

type SendSMSResult struct {
	MessageID    string
	Cost         float64
	RemainingBal float64
}

type DeliveryEvent struct {
	MessageID   string                `json:"message_id"`
	WalletID    int64                 `json:"wallet_id"`
	Destination smsDomain.Destination `json:"destination"`
	Line        smsDomain.LineType    `json:"line"`
	ChannelName string                `json:"channel_name"`
	CostUnits   int64                 `json:"cost_units"`
	CreatedAt   time.Time             `json:"created_at"`
}

type ChannelFinder interface {
	GetByName(ctx context.Context, name string) (*channel.Channel, error)
}

type SMSAcceptor interface {
	DeductAndEnqueue(ctx context.Context, walletID int64, amount wallet.Money, event DeliveryEvent, desc string) (*wallet.Wallet, error)
}

type SMSCostFinder interface {
	GetByLineType(ctx context.Context, lineType smsDomain.LineType) (*smsDomain.SMSCost, error)
}

type Service struct {
	channelRepo ChannelFinder
	acceptor    SMSAcceptor
	costRepo    SMSCostFinder
}

func NewService(channelRepo ChannelFinder, acceptor SMSAcceptor, costRepo SMSCostFinder) *Service {
	return &Service{channelRepo: channelRepo, acceptor: acceptor, costRepo: costRepo}
}

func (s *Service) Execute(ctx context.Context, cmd SendSMSCommand) (*SendSMSResult, error) {
	if !cmd.Line.IsValid() {
		return nil, fmt.Errorf("invalid line type: %s", cmd.Line)
	}
	if !cmd.Dest.IsValid() {
		return nil, fmt.Errorf("destination is required")
	}
	ch, err := s.channelRepo.GetByName(ctx, cmd.ChannelName)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	if !ch.IsActive {
		return nil, fmt.Errorf("channel '%s' is inactive", cmd.ChannelName)
	}
	costRecord, err := s.costRepo.GetByLineType(ctx, cmd.Line)
	if err != nil {
		return nil, fmt.Errorf("cost not found for line type '%s': %w", cmd.Line, err)
	}
	cost, err := wallet.NewMoney(costRecord.Cost)
	if err != nil || !cost.IsPositive() {
		return nil, fmt.Errorf("invalid configured cost")
	}
	messageID := generateMessageID()
	event := DeliveryEvent{MessageID: messageID, WalletID: ch.WalletID, Destination: cmd.Dest, Line: cmd.Line,
		ChannelName: cmd.ChannelName, CostUnits: cost.Units(), CreatedAt: time.Now().UTC()}
	w, err := s.acceptor.DeductAndEnqueue(ctx, ch.WalletID, cost, event,
		fmt.Sprintf("SMS via %s (%s) to %s", cmd.ChannelName, cmd.Line, cmd.Dest))
	if err != nil {
		return nil, fmt.Errorf("accept SMS: %w", err)
	}
	return &SendSMSResult{MessageID: messageID, Cost: cost.Float64(), RemainingBal: w.Balance.Float64()}, nil
}

func generateMessageID() string {
	return fmt.Sprintf("SMS-%d-%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}
