package sms

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"sms_gateway/internal/domain/channel"
	smsDomain "sms_gateway/internal/domain/sms"
	"sms_gateway/internal/domain/wallet"
)

var (
	ErrLineOverloaded     = errors.New("SMS line is at capacity")
	ErrInsufficientCredit = errors.New("insufficient wallet credit")
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
	DeadlineAt  time.Time             `json:"deadline_at,omitempty"`
}

// ReservationStore owns the fast, atomic credit reservation. PostgreSQL remains
// the source of truth and is updated only after the provider result is known.
type ReservationStore interface {
	Reserve(context.Context, int64, int64, string) (int64, error)
	IsReserved(context.Context, string) (bool, error)
	Commit(context.Context, string) error
	Refund(context.Context, int64, string) error
}

type ChannelFinder interface {
	GetByName(ctx context.Context, name string) (*channel.Channel, error)
}

type SMSAcceptor interface {
	DeductAndEnqueue(ctx context.Context, walletID int64, amount wallet.Money, event DeliveryEvent, desc string) (*wallet.Wallet, error)
}

type SMSEnqueuer interface {
	Enqueue(ctx context.Context, event DeliveryEvent, desc string) error
}

type SMSCostFinder interface {
	GetByLineType(ctx context.Context, lineType smsDomain.LineType) (*smsDomain.SMSCost, error)
}

type Service struct {
	channelRepo  ChannelFinder
	acceptor     any
	costRepo     SMSCostFinder
	expressSLA   time.Duration
	expressSlots chan struct{}
	normalSlots  chan struct{}
	reservations ReservationStore
}

func NewService(channelRepo ChannelFinder, acceptor SMSAcceptor, costRepo SMSCostFinder) *Service {
	return NewServiceWithPolicy(channelRepo, acceptor, costRepo, 5*time.Second, 100, 20)
}

func NewServiceWithExpressSLA(channelRepo ChannelFinder, acceptor SMSAcceptor, costRepo SMSCostFinder, expressSLA time.Duration) *Service {
	return NewServiceWithPolicy(channelRepo, acceptor, costRepo, expressSLA, 100, 20)
}

func NewServiceWithPolicy(channelRepo ChannelFinder, acceptor SMSAcceptor, costRepo SMSCostFinder, expressSLA time.Duration, expressLimit, normalLimit int) *Service {
	return &Service{channelRepo: channelRepo, acceptor: acceptor, costRepo: costRepo, expressSLA: expressSLA,
		expressSlots: make(chan struct{}, expressLimit), normalSlots: make(chan struct{}, normalLimit)}
}

func NewServiceWithReservation(channelRepo ChannelFinder, acceptor SMSEnqueuer, costRepo SMSCostFinder, reservations ReservationStore, expressSLA time.Duration, expressLimit, normalLimit int) *Service {
	return &Service{channelRepo: channelRepo, acceptor: acceptor, costRepo: costRepo, reservations: reservations,
		expressSLA: expressSLA, expressSlots: make(chan struct{}, expressLimit), normalSlots: make(chan struct{}, normalLimit)}
}

func (s *Service) Execute(ctx context.Context, cmd SendSMSCommand) (*SendSMSResult, error) {
	if !cmd.Line.IsValid() {
		return nil, fmt.Errorf("invalid line type: %s", cmd.Line)
	}
	if !cmd.Dest.IsValid() {
		return nil, fmt.Errorf("destination is required")
	}
	if !s.acquire(cmd.Line) {
		return nil, fmt.Errorf("%w: %s", ErrLineOverloaded, cmd.Line)
	}
	defer s.release(cmd.Line)
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
	now := time.Now().UTC()
	event := DeliveryEvent{MessageID: messageID, WalletID: ch.WalletID, Destination: cmd.Dest, Line: cmd.Line,
		ChannelName: cmd.ChannelName, CostUnits: cost.Units(), CreatedAt: now}
	if cmd.Line == smsDomain.LineTypeExpress {
		event.DeadlineAt = now.Add(s.expressSLA)
	}
	description := fmt.Sprintf("SMS via %s (%s) to %s", cmd.ChannelName, cmd.Line, cmd.Dest)
	if s.reservations != nil {
		remaining, err := s.reservations.Reserve(ctx, ch.WalletID, cost.Units(), messageID)
		if err != nil {
			return nil, fmt.Errorf("reserve SMS credit: %w", err)
		}
		enqueuer, ok := s.acceptor.(SMSEnqueuer)
		if !ok {
			_ = s.reservations.Refund(context.Background(), ch.WalletID, messageID)
			return nil, errors.New("SMS acceptor does not support reservation enqueue")
		}
		if err := enqueuer.Enqueue(ctx, event, description); err != nil {
			_ = s.reservations.Refund(context.Background(), ch.WalletID, messageID)
			return nil, fmt.Errorf("enqueue SMS: %w", err)
		}
		return &SendSMSResult{MessageID: messageID, Cost: cost.Float64(), RemainingBal: float64(remaining) / 10000}, nil
	}
	legacy, ok := s.acceptor.(SMSAcceptor)
	if !ok {
		return nil, errors.New("SMS acceptor does not support legacy enqueue")
	}
	w, err := legacy.DeductAndEnqueue(ctx, ch.WalletID, cost, event, description)
	if err != nil {
		return nil, fmt.Errorf("accept SMS: %w", err)
	}
	return &SendSMSResult{MessageID: messageID, Cost: cost.Float64(), RemainingBal: w.Balance.Float64()}, nil
}

func (s *Service) acquire(line smsDomain.LineType) bool {
	slots := s.normalSlots
	if line == smsDomain.LineTypeExpress {
		slots = s.expressSlots
	}
	select {
	case slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Service) release(line smsDomain.LineType) {
	if line == smsDomain.LineTypeExpress {
		<-s.expressSlots
		return
	}
	<-s.normalSlots
}

func generateMessageID() string {
	return fmt.Sprintf("SMS-%d-%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}
