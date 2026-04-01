package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStore manages subscriptions and notification state in Redis.
// Key patterns (from the plan):
//
//	user:{telegram_id}:cbr_subscriptions  -> Set of currency codes
//	user:{telegram_id}:crypto_subscriptions -> Set of symbols
type RedisStore struct {
	client *redis.Client
}

func NewRedis(addr string) *RedisStore {
	c := redis.NewClient(&redis.Options{Addr: addr})
	return &RedisStore{client: c}
}

func cbrKey(telegramID int64) string {
	return fmt.Sprintf("user:%d:cbr_subscriptions", telegramID)
}

func cryptoKey(telegramID int64) string {
	return fmt.Sprintf("user:%d:crypto_subscriptions", telegramID)
}

func (r *RedisStore) SubscribeCBR(ctx context.Context, telegramID int64, currency string) error {
	return r.client.SAdd(ctx, cbrKey(telegramID), currency).Err()
}

func (r *RedisStore) UnsubscribeCBR(ctx context.Context, telegramID int64, currency string) error {
	return r.client.SRem(ctx, cbrKey(telegramID), currency).Err()
}

func (r *RedisStore) GetCBRSubscriptions(ctx context.Context, telegramID int64) ([]string, error) {
	return r.client.SMembers(ctx, cbrKey(telegramID)).Result()
}

func (r *RedisStore) SubscribeCrypto(ctx context.Context, telegramID int64, symbol string) error {
	return r.client.SAdd(ctx, cryptoKey(telegramID), symbol).Err()
}

func (r *RedisStore) UnsubscribeCrypto(ctx context.Context, telegramID int64, symbol string) error {
	return r.client.SRem(ctx, cryptoKey(telegramID), symbol).Err()
}

func (r *RedisStore) GetCryptoSubscriptions(ctx context.Context, telegramID int64) ([]string, error) {
	return r.client.SMembers(ctx, cryptoKey(telegramID)).Result()
}

// GetAllCBRSubscribers returns map[currency_code][]telegramID
func (r *RedisStore) GetAllCBRSubscribers(ctx context.Context) (map[string][]int64, error) {
	// Scan all user:*:cbr_subscriptions keys
	result := make(map[string][]int64)
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, "user:*:cbr_subscriptions", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			var telegramID int64
			fmt.Sscanf(key, "user:%d:cbr_subscriptions", &telegramID)
			currencies, err := r.client.SMembers(ctx, key).Result()
			if err != nil {
				continue
			}
			for _, c := range currencies {
				result[c] = append(result[c], telegramID)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return result, nil
}

// GetAllCryptoSubscribers returns map[symbol][]telegramID
func (r *RedisStore) GetAllCryptoSubscribers(ctx context.Context) (map[string][]int64, error) {
	result := make(map[string][]int64)
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, "user:*:crypto_subscriptions", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			var telegramID int64
			fmt.Sscanf(key, "user:%d:crypto_subscriptions", &telegramID)
			symbols, err := r.client.SMembers(ctx, key).Result()
			if err != nil {
				continue
			}
			for _, s := range symbols {
				result[s] = append(result[s], telegramID)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return result, nil
}
