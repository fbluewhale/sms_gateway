package sms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
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
	CreditAndRecord(ctx context.Context, walletID int64, amount wallet.Money, refID string, desc string) (*wallet.Wallet, error)
}

type Sender interface {
	Send(ctx context.Context, dest smsDomain.Destination, line smsDomain.LineType, channel, messageID string) error
}

type Service struct {
	channelRepo   ChannelFinder
	walletPayable WalletPayable
	smsCostRepo   smsDomain.SMSCostRepository
	sender        Sender
	expressQueue  chan deliveryJob
	normalQueue   chan deliveryJob
	expressSlots  chan struct{}
	normalSlots   chan struct{}
	workerCtx     context.Context
	cancelWorkers context.CancelFunc
	queueMu       sync.RWMutex
	closed        bool
	closeOnce     sync.Once
	workers       sync.WaitGroup
}

type deliveryJob struct {
	dest        smsDomain.Destination
	line        smsDomain.LineType
	channelName string
	messageID   string
	walletID    int64
	cost        wallet.Money
}

const (
	queueSize     = 100
	refundTimeout = 5 * time.Second
)

func NewService(
	channelRepo ChannelFinder,
	walletPayable WalletPayable,
	smsCostRepo smsDomain.SMSCostRepository,
	sender Sender,
) *Service {
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	s := &Service{
		channelRepo:   channelRepo,
		walletPayable: walletPayable,
		smsCostRepo:   smsCostRepo,
		sender:        sender,
		expressQueue:  make(chan deliveryJob, queueSize),
		normalQueue:   make(chan deliveryJob, queueSize),
		expressSlots:  make(chan struct{}, queueSize),
		normalSlots:   make(chan struct{}, queueSize),
		workerCtx:     workerCtx,
		cancelWorkers: cancelWorkers,
	}
	s.workers.Add(2)
	go s.runWorker(s.expressQueue, s.expressSlots)
	go s.runWorker(s.normalQueue, s.normalSlots)
	return s
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
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()
	if s.closed {
		return nil, errors.New("SMS service is shutting down")
	}
	queue, slots, err := s.queueFor(cmd.Line)
	if err != nil {
		return nil, err
	}
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("reserve SMS queue: %w", ctx.Err())
	}

	walletEntity, err := s.walletPayable.DeductAndRecord(ctx,
		channelEntity.WalletID,
		cost,
		msgID,
		fmt.Sprintf("SMS via %s (%s) to %s", cmd.ChannelName, cmd.Line, cmd.Dest),
	)
	if err != nil {
		<-slots
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	job := deliveryJob{
		dest:        cmd.Dest,
		line:        cmd.Line,
		channelName: cmd.ChannelName,
		messageID:   msgID,
		walletID:    channelEntity.WalletID,
		cost:        cost,
	}
	queue <- job

	return &SendSMSResult{
		MessageID:    msgID,
		Cost:         cost.Float64(),
		RemainingBal: walletEntity.Balance.Float64(),
	}, nil
}

func (s *Service) queueFor(line smsDomain.LineType) (chan deliveryJob, chan struct{}, error) {
	switch line {
	case smsDomain.LineTypeExpress:
		return s.expressQueue, s.expressSlots, nil
	case smsDomain.LineTypeNormal:
		return s.normalQueue, s.normalSlots, nil
	default:
		return nil, nil, fmt.Errorf("unsupported line type: %s", line)
	}
}

func (s *Service) runWorker(queue <-chan deliveryJob, slots chan struct{}) {
	defer s.workers.Done()
	for job := range queue {
		if err := s.sender.Send(s.workerCtx, job.dest, job.line, job.channelName, job.messageID); err != nil {
			slog.Error("send queued SMS", "error", err, "message_id", job.messageID, "line", job.line, "channel", job.channelName)
			s.refund(job)
		}
		<-slots
	}
}

func (s *Service) refund(job deliveryJob) {
	ctx, cancel := context.WithTimeout(context.Background(), refundTimeout)
	defer cancel()
	_, err := s.walletPayable.CreditAndRecord(
		ctx,
		job.walletID,
		job.cost,
		job.messageID,
		fmt.Sprintf("Refund for failed SMS via %s (%s) to %s", job.channelName, job.line, job.dest),
	)
	if err != nil {
		slog.Error("refund failed SMS", "error", err, "message_id", job.messageID, "wallet_id", job.walletID, "amount", job.cost)
		return
	}
	slog.Info("refunded failed SMS", "message_id", job.messageID, "wallet_id", job.walletID, "amount", job.cost)
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.closeOnce.Do(func() {
		s.queueMu.Lock()
		s.closed = true
		close(s.expressQueue)
		close(s.normalQueue)
		s.queueMu.Unlock()
	})

	done := make(chan struct{})
	go func() {
		s.workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.cancelWorkers()
		return nil
	case <-ctx.Done():
		s.cancelWorkers()
		return fmt.Errorf("shutdown SMS workers: %w", ctx.Err())
	}
}

func generateMessageID() string {
	return fmt.Sprintf("SMS-%d-%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}
