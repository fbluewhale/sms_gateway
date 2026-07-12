package tests

import (
	"testing"
	"time"

	"sms_gateway/internal/application/admin"
	"sms_gateway/internal/domain/wallet"
)

func TestToWalletResponseFormatsTimestampsAsUTC(t *testing.T) {
	location := time.FixedZone("UTC+03:30", 3*60*60+30*60)
	createdAt := time.Date(2026, time.July, 12, 10, 34, 46, 0, location)

	response := admin.ToWalletResponse(&wallet.Wallet{
		Balance:   wallet.MustMoney(10),
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	})

	const expected = "2026-07-12T07:04:46Z"
	if response.CreatedAt != expected || response.UpdatedAt != expected {
		t.Fatalf("timestamps = %q, %q; want %q", response.CreatedAt, response.UpdatedAt, expected)
	}
}

func TestToTransactionResponseFormatsTimestampAsUTC(t *testing.T) {
	location := time.FixedZone("UTC+03:30", 3*60*60+30*60)
	createdAt := time.Date(2026, time.July, 12, 10, 34, 46, 0, location)

	response := admin.ToTransactionResponse(wallet.WalletTransaction{CreatedAt: createdAt})

	const expected = "2026-07-12T07:04:46Z"
	if response.CreatedAt != expected {
		t.Fatalf("created_at = %q; want %q", response.CreatedAt, expected)
	}
}
