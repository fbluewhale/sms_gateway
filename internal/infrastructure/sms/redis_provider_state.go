package sms

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisCircuitAcquireScript = `local t = redis.call('TIME')
local now = (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000)
local state = redis.call('HGET', KEYS[1], 'state') or 'closed'
local generation = tonumber(redis.call('HGET', KEYS[1], 'generation') or '0')
if state == 'open' then
  local opened_at = tonumber(redis.call('HGET', KEYS[1], 'opened_at') or '0')
  if now - opened_at < tonumber(ARGV[1]) then
    return {0, generation, 1, 0}
  end
  redis.call('HSET', KEYS[1], 'state', 'half-open', 'half_open_until', now + tonumber(ARGV[2]))
  return {1, generation, 2, 1}
end
if state == 'half-open' then
  local lease_until = tonumber(redis.call('HGET', KEYS[1], 'half_open_until') or '0')
  if lease_until > now then
    return {0, generation, 2, 0}
  end
  generation = generation + 1
  redis.call('HSET', KEYS[1], 'generation', generation, 'half_open_until', now + tonumber(ARGV[2]))
  return {1, generation, 2, 0}
end
return {1, generation, 0, 0}`

const redisCircuitCompleteScript = `local t = redis.call('TIME')
local now = (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000)
local generation = tonumber(redis.call('HGET', KEYS[1], 'generation') or '0')
if generation ~= tonumber(ARGV[1]) then
  local stale_state = redis.call('HGET', KEYS[1], 'state') or 'closed'
  local stale_code = 0
  if stale_state == 'open' then stale_code = 1 elseif stale_state == 'half-open' then stale_code = 2 end
  return {stale_code, 0}
end
local state = redis.call('HGET', KEYS[1], 'state') or 'closed'
local success = tonumber(ARGV[2])
if state == 'half-open' then
  generation = generation + 1
  if success == 1 then
    redis.call('HSET', KEYS[1], 'state', 'closed', 'failures', 0, 'generation', generation)
    redis.call('HDEL', KEYS[1], 'opened_at', 'half_open_until')
    return {0, 1}
  end
  redis.call('HSET', KEYS[1], 'state', 'open', 'opened_at', now, 'generation', generation)
  redis.call('HDEL', KEYS[1], 'half_open_until')
  return {1, 1}
end
if state == 'closed' then
  if success == 1 then
    redis.call('HSET', KEYS[1], 'failures', 0)
    return {0, 0}
  end
  local failures = redis.call('HINCRBY', KEYS[1], 'failures', 1)
  if failures >= tonumber(ARGV[3]) then
    generation = generation + 1
    redis.call('HSET', KEYS[1], 'state', 'open', 'opened_at', now, 'generation', generation)
    return {1, 1}
  end
end
local code = 0
if state == 'open' then code = 1 elseif state == 'half-open' then code = 2 end
return {code, 0}`

type redisProviderState struct {
	client           *redis.Client
	prefix           string
	failureThreshold int
	cooldown         time.Duration
	halfOpenLease    time.Duration
}

func newRedisProviderState(ctx context.Context, redisURL, poolName string, failureThreshold int, cooldown, halfOpenLease time.Duration) (*redisProviderState, error) {
	if poolName == "" {
		return nil, fmt.Errorf("Redis provider pool name is required")
	}
	if halfOpenLease <= 0 {
		return nil, fmt.Errorf("circuit breaker half-open lease must be positive")
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse provider pool Redis URL: %w", err)
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect provider pool Redis: %w", err)
	}
	encodedPool := base64.RawURLEncoding.EncodeToString([]byte(poolName))
	return &redisProviderState{
		client: client, prefix: "sms:provider-pool:" + encodedPool,
		failureThreshold: failureThreshold, cooldown: cooldown, halfOpenLease: halfOpenLease,
	}, nil
}

func (s *redisProviderState) next(ctx context.Context) (uint64, error) {
	value, err := s.client.Incr(ctx, s.prefix+":round-robin").Uint64()
	if err != nil {
		return 0, err
	}
	return value - 1, nil
}

func (s *redisProviderState) acquire(ctx context.Context, provider string) (uint64, bool, circuitState, bool, error) {
	result, err := s.client.Eval(ctx, redisCircuitAcquireScript, []string{s.circuitKey(provider)}, s.cooldown.Milliseconds(), s.halfOpenLease.Milliseconds()).Slice()
	if err != nil {
		return 0, false, stateClosed, false, err
	}
	if len(result) != 4 {
		return 0, false, stateClosed, false, fmt.Errorf("unexpected circuit acquire result length %d", len(result))
	}
	allowed, err := redisInt64(result[0])
	if err != nil {
		return 0, false, stateClosed, false, err
	}
	generation, err := redisInt64(result[1])
	if err != nil {
		return 0, false, stateClosed, false, err
	}
	state, err := redisCircuitState(result[2])
	if err != nil {
		return 0, false, stateClosed, false, err
	}
	changed, err := redisInt64(result[3])
	return uint64(generation), allowed == 1, state, changed == 1, err
}

func (s *redisProviderState) complete(ctx context.Context, provider string, generation uint64, success bool) (circuitState, bool, error) {
	succeeded := 0
	if success {
		succeeded = 1
	}
	result, err := s.client.Eval(ctx, redisCircuitCompleteScript, []string{s.circuitKey(provider)}, generation, succeeded, s.failureThreshold).Slice()
	if err != nil {
		return stateClosed, false, err
	}
	if len(result) != 2 {
		return stateClosed, false, fmt.Errorf("unexpected circuit completion result length %d", len(result))
	}
	state, err := redisCircuitState(result[0])
	if err != nil {
		return stateClosed, false, err
	}
	changed, err := redisInt64(result[1])
	return state, changed == 1, err
}

func (s *redisProviderState) circuitKey(provider string) string {
	encodedProvider := base64.RawURLEncoding.EncodeToString([]byte(provider))
	return s.prefix + ":circuit:" + encodedProvider
}

func (s *redisProviderState) close() error { return s.client.Close() }

func redisCircuitState(value any) (circuitState, error) {
	code, err := redisInt64(value)
	if err != nil {
		return stateClosed, err
	}
	switch code {
	case 0:
		return stateClosed, nil
	case 1:
		return stateOpen, nil
	case 2:
		return stateHalfOpen, nil
	default:
		return stateClosed, fmt.Errorf("unknown Redis circuit state %d", code)
	}
}

func redisInt64(value any) (int64, error) {
	switch value := value.(type) {
	case int64:
		return value, nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse Redis integer %q: %w", value, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}

func NewRedisRoundRobinSender(ctx context.Context, redisURL, poolName string, providers []Provider, failureThreshold int, cooldown, halfOpenLease time.Duration, logger *slog.Logger) (*RoundRobinSender, error) {
	providers, logger, err := validatePool(providers, failureThreshold, cooldown, logger)
	if err != nil {
		return nil, err
	}
	state, err := newRedisProviderState(ctx, redisURL, poolName, failureThreshold, cooldown, halfOpenLease)
	if err != nil {
		return nil, err
	}
	return newRoundRobinSender(providers, state, logger), nil
}

func NewDefaultRedisMockRoundRobinSender(ctx context.Context, redisURL string, logger *slog.Logger, failureThreshold int, cooldown, halfOpenLease time.Duration) (*RoundRobinSender, error) {
	providers, err := defaultMockProviders(logger)
	if err != nil {
		return nil, err
	}
	return NewRedisRoundRobinSender(ctx, redisURL, "global", providers, failureThreshold, cooldown, halfOpenLease, logger)
}
