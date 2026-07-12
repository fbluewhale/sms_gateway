package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sms_gateway/internal/domain/wallet"
)

type PostgresWalletRepository struct {
	db *gorm.DB
}

func NewPostgresWalletRepository(db *gorm.DB) *PostgresWalletRepository {
	return &PostgresWalletRepository{db: db}
}

func (r *PostgresWalletRepository) GetByID(ctx context.Context, id int64) (*wallet.Wallet, error) {
	var model WalletModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}
	return toWalletModel(&model), nil
}

func (r *PostgresWalletRepository) Create(ctx context.Context, w *wallet.Wallet) error {
	model := WalletModel{
		Balance: w.Balance.Float64(),
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("insert wallet: %w", err)
	}
	w.ID = model.ID
	w.CreatedAt = model.CreatedAt
	w.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *PostgresWalletRepository) CreditAndRecord(ctx context.Context, walletID int64, amount wallet.Money, refID, desc string) (*wallet.Wallet, error) {
	return r.mutateAndRecord(ctx, walletID, amount, refID, desc, wallet.TransactionTypeCredit)
}

func (r *PostgresWalletRepository) DeductAndRecord(ctx context.Context, walletID int64, amount wallet.Money, refID string, desc string) (*wallet.Wallet, error) {
	return r.mutateAndRecord(ctx, walletID, amount, refID, desc, wallet.TransactionTypeDebit)
}

func (r *PostgresWalletRepository) mutateAndRecord(ctx context.Context, walletID int64, amount wallet.Money, refID, desc string, typ wallet.TransactionType) (*wallet.Wallet, error) {
	var result *wallet.Wallet

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model WalletModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&model, walletID).Error; err != nil {
			return fmt.Errorf("wallet not found: %w", err)
		}

		w := toWalletModel(&model)
		if typ == wallet.TransactionTypeDebit && !w.CanAfford(amount) {
			return fmt.Errorf("%w: have %s, need %s", wallet.ErrInsufficientFunds, w.Balance, amount)
		}
		if typ == wallet.TransactionTypeDebit {
			w.Deduct(amount)
		} else {
			w.Credit(amount)
		}

		if err := tx.Model(&WalletModel{}).Where("id = ?", walletID).
			Update("balance", w.Balance.Float64()).Error; err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		txModel := WalletTransactionModel{
			WalletID:    walletID,
			Amount:      amount.Float64(),
			Type:        string(typ),
			Description: desc,
			ReferenceID: refID,
		}
		if typ == wallet.TransactionTypeDebit {
			txModel.Amount = -txModel.Amount
		}
		if err := tx.Create(&txModel).Error; err != nil {
			return fmt.Errorf("record transaction: %w", err)
		}

		result = w
		return nil
	})

	return result, err
}

func (r *PostgresWalletRepository) GetTransactionsByWalletID(ctx context.Context, walletID int64) ([]wallet.WalletTransaction, error) {
	var models []WalletTransactionModel
	if err := r.db.WithContext(ctx).Where("wallet_id = ?", walletID).
		Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list wallet transactions: %w", err)
	}
	txs := make([]wallet.WalletTransaction, 0, len(models))
	for _, m := range models {
		txs = append(txs, toWalletTransactionModel(&m))
	}
	return txs, nil
}
