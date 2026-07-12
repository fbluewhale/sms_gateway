package wallet

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type Money struct {
	units int64
}

const scale = int64(10_000)

func NewMoney(amount float64) (Money, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > float64(math.MaxInt64)/float64(scale) {
		return Money{}, ErrInvalidAmount
	}
	return Money{units: int64(math.Round(amount * float64(scale)))}, nil
}

func MustMoney(amount float64) Money {
	m, err := NewMoney(amount)
	if err != nil {
		panic(err)
	}
	return m
}

func MoneyFromUnits(units int64) (Money, error) {
	if units < 0 {
		return Money{}, ErrInvalidAmount
	}
	return Money{units: units}, nil
}

func (m Money) Units() int64                 { return m.units }
func (m Money) Float64() float64             { return float64(m.units) / float64(scale) }
func (m Money) String() string               { return fmt.Sprintf("%.4f", m.Float64()) }
func (m Money) IsPositive() bool             { return m.units > 0 }
func (m Money) Equals(other Money) bool      { return m.units == other.units }
func (m Money) GreaterThan(other Money) bool { return m.units > other.units }

func (m Money) Subtract(other Money) (Money, error) {
	if other.units > m.units {
		return Money{}, ErrInsufficientFunds
	}
	return Money{units: m.units - other.units}, nil
}

func (m Money) Add(other Money) Money { return Money{units: m.units + other.units} }

type TransactionType string

const (
	TransactionTypeCredit TransactionType = "credit"
	TransactionTypeDebit  TransactionType = "debit"
)

type Wallet struct {
	ID        int64
	Balance   Money
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WalletTransaction struct {
	ID          int64
	WalletID    int64
	Amount      float64
	Type        TransactionType
	Description string
	ReferenceID string
	CreatedAt   time.Time
}

func (w *Wallet) CanAfford(cost Money) bool {
	return w.Balance.GreaterThan(cost) || w.Balance.Equals(cost)
}

func (w *Wallet) Deduct(amount Money) {
	w.Balance, _ = w.Balance.Subtract(amount)
}

func (w *Wallet) Credit(amount Money) {
	w.Balance = w.Balance.Add(amount)
}
