package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sms_gateway/internal/config"
	"sms_gateway/internal/domain/wallet"
	"sms_gateway/internal/infrastructure/persistence"
)

func TestConcurrentDebitsDoNotOverdrawWallet(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL concurrency tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := persistence.Connect(ctx, config.DatabaseConfig{
		Host:     envOr("TEST_DB_HOST", "127.0.0.1"),
		Port:     envOr("TEST_DB_PORT", "5432"),
		User:     envOr("TEST_DB_USER", "postgres"),
		Password: envOr("TEST_DB_PASSWORD", "postgres"),
		Name:     envOr("TEST_DB_NAME", "sms_gateway"),
		SSLMode:  envOr("TEST_DB_SSLMODE", "disable"),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := persistence.NewPostgresWalletRepository(db)
	w := &wallet.Wallet{Balance: wallet.MustMoney(5)}
	if err := repo.Create(ctx, w); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM wallet_transactions WHERE wallet_id = ?", w.ID)
		db.Exec("DELETE FROM wallets WHERE id = ?", w.ID)
	})

	const attempts = 20
	var successes atomic.Int32
	errs := make(chan error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, err := repo.DeductAndRecord(ctx, w.ID, wallet.MustMoney(1), fmt.Sprintf("concurrent-%d", index), "concurrency test")
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, wallet.ErrInsufficientFunds):
			default:
				errs <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("unexpected debit error: %v", err)
	}

	if got := successes.Load(); got != 5 {
		t.Fatalf("successful debits = %d; want 5", got)
	}
	gotWallet, err := repo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotWallet.Balance.Units() != 0 {
		t.Fatalf("final balance = %s; want 0.0000", gotWallet.Balance)
	}
	transactions, err := repo.GetTransactionsByWalletID(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 5 {
		t.Fatalf("transaction count = %d; want 5", len(transactions))
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
