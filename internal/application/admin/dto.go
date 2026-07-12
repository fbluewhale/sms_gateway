package admin

import (
	"sms_gateway/internal/domain/channel"
	"sms_gateway/internal/domain/wallet"
	"time"
)

type CreateWalletRequest struct {
	Balance float64 `json:"balance"`
}

type TopUpWalletRequest struct {
	Amount      float64 `json:"amount" binding:"required"`
	ReferenceID string  `json:"reference_id"`
}

type CreateChannelRequest struct {
	Name     string `json:"name" binding:"required"`
	WalletID int64  `json:"wallet_id" binding:"required"`
}

type WalletResponse struct {
	ID        int64   `json:"id"`
	Balance   float64 `json:"balance"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type ChannelResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	WalletID  int64  `json:"wallet_id"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TransactionResponse struct {
	ID          int64   `json:"id"`
	WalletID    int64   `json:"wallet_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	ReferenceID string  `json:"reference_id"`
	CreatedAt   string  `json:"created_at"`
}

func ToWalletResponse(w *wallet.Wallet) WalletResponse {
	return WalletResponse{
		ID:        w.ID,
		Balance:   w.Balance.Float64(),
		CreatedAt: w.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: w.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ToTransactionResponse(t wallet.WalletTransaction) TransactionResponse {
	return TransactionResponse{
		ID:          t.ID,
		WalletID:    t.WalletID,
		Amount:      t.Amount,
		Type:        string(t.Type),
		Description: t.Description,
		ReferenceID: t.ReferenceID,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func ToChannelResponse(ch channel.Channel) ChannelResponse {
	return ChannelResponse{ID: ch.ID, Name: ch.Name, WalletID: ch.WalletID, IsActive: ch.IsActive,
		CreatedAt: ch.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: ch.UpdatedAt.UTC().Format(time.RFC3339)}
}
