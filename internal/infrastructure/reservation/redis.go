package reservation

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	app "sms_gateway/internal/application/sms"
	"sms_gateway/internal/domain/wallet"
)

var ErrInsufficientCredit = app.ErrInsufficientCredit

const (
	reserveScript = `local available = redis.call('GET', KEYS[1])
if not available then return -2 end
if redis.call('EXISTS', KEYS[2]) == 1 then return -3 end
local amount = tonumber(ARGV[1])
if tonumber(available) < amount then return -1 end
redis.call('DECRBY', KEYS[1], amount)
redis.call('SET', KEYS[2], ARGV[1])
return tonumber(available) - amount`
	commitScript = `return redis.call('DEL', KEYS[1])`
	refundScript = `local amount = redis.call('GET', KEYS[1])
if not amount then return 0 end
redis.call('INCRBY', KEYS[2], amount)
redis.call('DEL', KEYS[1])
return 1`
)

type BalanceLoader interface {
	GetByID(context.Context, int64) (*wallet.Wallet, error)
}

type Store struct {
	client *redis.Client
	loader BalanceLoader
}

func NewStore(url string, loader BalanceLoader) (*Store, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}
	return &Store{client: redis.NewClient(options), loader: loader}, nil
}

func (s *Store) Close() error { return s.client.Close() }

func balanceKey(walletID int64) string { return fmt.Sprintf("sms:wallet:%d:available", walletID) }
func reservationKey(id string) string  { return "sms:reservation:" + id }

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) Reserve(ctx context.Context, walletID, amount int64, messageID string) (int64, error) {
	result, err := s.client.Eval(ctx, reserveScript, []string{balanceKey(walletID), reservationKey(messageID)}, amount).Int64()
	if err != nil {
		return 0, fmt.Errorf("reserve credit in Redis: %w", err)
	}
	switch result {
	case -1:
		return 0, app.ErrInsufficientCredit
	case -2:
		if s.loader == nil {
			return 0, fmt.Errorf("wallet %d is not initialized in Redis", walletID)
		}
		w, loadErr := s.loader.GetByID(ctx, walletID)
		if loadErr != nil {
			return 0, fmt.Errorf("load wallet %d for Redis initialization: %w", walletID, loadErr)
		}
		if err := s.client.SetNX(ctx, balanceKey(walletID), w.Balance.Units(), 0).Err(); err != nil {
			return 0, fmt.Errorf("initialize wallet %d in Redis: %w", walletID, err)
		}
		return s.Reserve(ctx, walletID, amount, messageID)
	case -3:
		return 0, fmt.Errorf("reservation already exists: %s", messageID)
	default:
		return result, nil
	}
}

func (s *Store) Commit(ctx context.Context, messageID string) error {
	return s.client.Eval(ctx, commitScript, []string{reservationKey(messageID)}).Err()
}

func (s *Store) IsReserved(ctx context.Context, messageID string) (bool, error) {
	count, err := s.client.Exists(ctx, reservationKey(messageID)).Result()
	return count > 0, err
}

func (s *Store) Refund(ctx context.Context, walletID int64, messageID string) error {
	return s.client.Eval(ctx, refundScript, []string{reservationKey(messageID), balanceKey(walletID)}).Err()
}

func (s *Store) SetAvailable(ctx context.Context, walletID, units int64) error {
	return s.client.Set(ctx, balanceKey(walletID), strconv.FormatInt(units, 10), 0).Err()
}
