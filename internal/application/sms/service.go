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

type ChannelFinder interface {
	GetByName(ctx context.Context, name string) (*channel.Channel, error)
}

type WalletPayable interface {
	DeductAndRecord(ctx context.Context, walletID int64, amount wallet.Money, refID string, desc string) (*wallet.Wallet, error)
}

type Sender interface {
	Send(ctx context.Context, dest smsDomain.Destination, line smsDomain.LineType, channel, messageID string) error
}

type Service struct {
	channelRepo   ChannelFinder
	walletPayable WalletPayable
	smsCostRepo   smsDomain.SMSCostRepository
	sender        Sender
}

func NewService(
	channelRepo ChannelFinder,
	walletPayable WalletPayable,
	smsCostRepo smsDomain.SMSCostRepository,
	sender Sender,
) *Service {
	return &Service{
		channelRepo:   channelRepo,
		walletPayable: walletPayable,
		smsCostRepo:   smsCostRepo,
		sender:        sender,
	}
}

func (s *Service) Execute(ctx context.Context, cmd SendSMSCommand) (*SendSMSResult, error) {
	if !cmd.Line.IsValid() {
		return nil, fmt.Errorf("invalid line type: %s", cmd.Line)
	}

	if !cmd.Dest.IsValid() {
		return nil, fmt.Errorf("destination is required")
	}

	channelEntity, err := s.channelRepo.GetByName(ctx, cmd.ChannelName)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	if !channelEntity.IsActive {
		return nil, fmt.Errorf("channel '%s' is inactive", cmd.ChannelName)
	}

	costRecord, err := s.smsCostRepo.GetByLineType(ctx, cmd.Line)
	if err != nil {
		return nil, fmt.Errorf("cost not found for line type '%s': %w", cmd.Line, err)
	}

	cost, err := wallet.NewMoney(costRecord.Cost)
	if err != nil || !cost.IsPositive() {
		return nil, fmt.Errorf("invalid configured cost")
	}
	msgID := generateMessageID()

	walletEntity, err := s.walletPayable.DeductAndRecord(ctx,
		channelEntity.WalletID,
		cost,
		msgID,
		fmt.Sprintf("SMS via %s (%s) to %s", cmd.ChannelName, cmd.Line, cmd.Dest),
	)
	if err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	if err := s.sender.Send(ctx, cmd.Dest, cmd.Line, cmd.ChannelName, msgID); err != nil {
		return nil, fmt.Errorf("send SMS: %w", err)
	}

	return &SendSMSResult{
		MessageID:    msgID,
		Cost:         cost.Float64(),
		RemainingBal: walletEntity.Balance.Float64(),
	}, nil
}

func generateMessageID() string {
	return fmt.Sprintf("SMS-%d-%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}
