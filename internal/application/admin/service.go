package admin

import (
	"context"
	"errors"
	"fmt"
	"sms_gateway/internal/domain/channel"
	"sms_gateway/internal/domain/wallet"
	"strings"
)

var ErrInvalidInput = errors.New("invalid input")

type AdminService struct {
	walletRepo  wallet.WalletRepository
	channelRepo channel.ChannelRepository
}

func NewAdminService(
	walletRepo wallet.WalletRepository,
	channelRepo channel.ChannelRepository,
) *AdminService {
	return &AdminService{
		walletRepo:  walletRepo,
		channelRepo: channelRepo,
	}
}

func (s *AdminService) CreateWallet(ctx context.Context, initialBalance float64) (*wallet.Wallet, error) {
	m, err := wallet.NewMoney(initialBalance)
	if err != nil {
		return nil, fmt.Errorf("%w: balance", ErrInvalidInput)
	}
	w := &wallet.Wallet{
		Balance: m,
	}
	if err := s.walletRepo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}
	return w, nil
}

func (s *AdminService) GetWallet(ctx context.Context, id int64) (*wallet.Wallet, error) {
	return s.walletRepo.GetByID(ctx, id)
}

func (s *AdminService) TopUpWallet(ctx context.Context, walletID int64, amount float64, referenceID string) (*wallet.Wallet, error) {
	m, err := wallet.NewMoney(amount)
	if err != nil || !m.IsPositive() {
		return nil, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	return s.walletRepo.CreditAndRecord(ctx, walletID, m, strings.TrimSpace(referenceID), "Wallet top-up")
}

func (s *AdminService) CreateChannel(ctx context.Context, name string, walletID int64) (*channel.Channel, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 || walletID <= 0 {
		return nil, fmt.Errorf("%w: invalid channel", ErrInvalidInput)
	}
	ch := &channel.Channel{
		Name:     name,
		WalletID: walletID,
		IsActive: true,
	}
	if err := s.channelRepo.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return ch, nil
}

func (s *AdminService) ListChannels(ctx context.Context) ([]channel.Channel, error) {
	return s.channelRepo.List(ctx)
}

func (s *AdminService) GetWalletTransactions(ctx context.Context, walletID int64) ([]wallet.WalletTransaction, error) {
	return s.walletRepo.GetTransactionsByWalletID(ctx, walletID)
}
