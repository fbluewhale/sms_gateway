package wallet

import "context"

type WalletRepository interface {
	GetByID(ctx context.Context, id int64) (*Wallet, error)
	Create(ctx context.Context, wallet *Wallet) error
	CreditAndRecord(ctx context.Context, walletID int64, amount Money, refID, desc string) (*Wallet, error)
	DeductAndRecord(ctx context.Context, walletID int64, amount Money, refID, desc string) (*Wallet, error)
	GetTransactionsByWalletID(ctx context.Context, walletID int64) ([]WalletTransaction, error)
}
