package tests

import (
	"errors"
	"math"
	"testing"

	"sms_gateway/internal/domain/wallet"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		wantUnits int64
		wantErr   bool
	}{
		{"zero", 0, 0, false}, {"four decimals", 1.2345, 12345, false},
		{"rounds", 1.23456, 12346, false}, {"negative", -1, 0, true},
		{"nan", math.NaN(), 0, true}, {"infinity", math.Inf(1), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wallet.NewMoney(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewMoney() error = %v", err)
			}
			if got.Units() != tt.wantUnits {
				t.Fatalf("units = %d, want %d", got.Units(), tt.wantUnits)
			}
		})
	}
}

func TestMoneySubtract(t *testing.T) {
	_, err := wallet.MustMoney(1).Subtract(wallet.MustMoney(2))
	if !errors.Is(err, wallet.ErrInsufficientFunds) {
		t.Fatalf("error = %v", err)
	}
}
