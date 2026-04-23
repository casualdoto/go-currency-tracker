package store

import (
	"context"
	"sort"
	"testing"
)

// ─── pure function tests (no Redis required) ──────────────────────────────────

func TestCBRKey_format(t *testing.T) {
	got := cbrKey(12345)
	want := "user:12345:cbr_subscriptions"
	if got != want {
		t.Errorf("cbrKey(12345) = %q, want %q", got, want)
	}
}

func TestCryptoKey_format(t *testing.T) {
	got := cryptoKey(99)
	want := "user:99:crypto_subscriptions"
	if got != want {
		t.Errorf("cryptoKey(99) = %q, want %q", got, want)
	}
}

func TestCBRKey_zeroID(t *testing.T) {
	got := cbrKey(0)
	want := "user:0:cbr_subscriptions"
	if got != want {
		t.Errorf("cbrKey(0) = %q, want %q", got, want)
	}
}

func TestCryptoKey_largeID(t *testing.T) {
	got := cryptoKey(9999999999)
	want := "user:9999999999:crypto_subscriptions"
	if got != want {
		t.Errorf("cryptoKey(9999999999) = %q, want %q", got, want)
	}
}

// ─── Redis integration tests (skipped when Redis is unavailable) ──────────────

// newTestStore returns a RedisStore connected to localhost:6379.
// The test is skipped if Redis is not available.
func newTestStore(t *testing.T) *RedisStore {
	t.Helper()
	s := NewRedis("localhost:6379")
	if err := s.client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
	return s
}

func TestRedisStore_SubscribeAndGetCBR(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	defer s.client.Del(ctx, cbrKey(1001))

	if err := s.SubscribeCBR(ctx, 1001, "USD"); err != nil {
		t.Fatalf("SubscribeCBR: %v", err)
	}

	subs, err := s.GetCBRSubscriptions(ctx, 1001)
	if err != nil {
		t.Fatalf("GetCBRSubscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0] != "USD" {
		t.Errorf("expected [USD], got %v", subs)
	}
}

func TestRedisStore_SubscribeMultiple_CBR(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	defer s.client.Del(ctx, cbrKey(1002))

	currencies := []string{"USD", "EUR", "GBP"}
	for _, c := range currencies {
		if err := s.SubscribeCBR(ctx, 1002, c); err != nil {
			t.Fatalf("SubscribeCBR(%s): %v", c, err)
		}
	}

	subs, err := s.GetCBRSubscriptions(ctx, 1002)
	if err != nil {
		t.Fatalf("GetCBRSubscriptions: %v", err)
	}
	sort.Strings(subs)
	sort.Strings(currencies)
	if len(subs) != len(currencies) {
		t.Errorf("expected %v, got %v", currencies, subs)
	}
}

func TestRedisStore_UnsubscribeCBR(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	defer s.client.Del(ctx, cbrKey(1003))

	_ = s.SubscribeCBR(ctx, 1003, "USD")
	_ = s.SubscribeCBR(ctx, 1003, "EUR")

	if err := s.UnsubscribeCBR(ctx, 1003, "USD"); err != nil {
		t.Fatalf("UnsubscribeCBR: %v", err)
	}

	subs, _ := s.GetCBRSubscriptions(ctx, 1003)
	for _, c := range subs {
		if c == "USD" {
			t.Error("USD should have been unsubscribed")
		}
	}
	found := false
	for _, c := range subs {
		if c == "EUR" {
			found = true
		}
	}
	if !found {
		t.Error("EUR should still be subscribed")
	}
}

func TestRedisStore_SubscribeAndGetCrypto(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	defer s.client.Del(ctx, cryptoKey(2001))

	if err := s.SubscribeCrypto(ctx, 2001, "BTCUSDT"); err != nil {
		t.Fatalf("SubscribeCrypto: %v", err)
	}

	subs, err := s.GetCryptoSubscriptions(ctx, 2001)
	if err != nil {
		t.Fatalf("GetCryptoSubscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0] != "BTCUSDT" {
		t.Errorf("expected [BTCUSDT], got %v", subs)
	}
}

func TestRedisStore_UnsubscribeCrypto(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	defer s.client.Del(ctx, cryptoKey(2002))

	_ = s.SubscribeCrypto(ctx, 2002, "BTCUSDT")
	_ = s.SubscribeCrypto(ctx, 2002, "ETHUSDT")

	if err := s.UnsubscribeCrypto(ctx, 2002, "BTCUSDT"); err != nil {
		t.Fatalf("UnsubscribeCrypto: %v", err)
	}

	subs, _ := s.GetCryptoSubscriptions(ctx, 2002)
	for _, sym := range subs {
		if sym == "BTCUSDT" {
			t.Error("BTCUSDT should have been unsubscribed")
		}
	}
}

func TestRedisStore_EmptySubscriptions_returnsNil(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	subs, err := s.GetCBRSubscriptions(ctx, 99999)
	if err != nil {
		t.Fatalf("GetCBRSubscriptions for empty: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected empty, got %v", subs)
	}
}
