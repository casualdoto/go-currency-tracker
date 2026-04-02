package storage

import (
	"testing"
	"time"
)

// Tests for the shared domain types — CurrencyRate and CryptoRate.
// These are pure struct tests that need no external dependencies.

func TestCurrencyRate_fields(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	r := CurrencyRate{
		ID:           1,
		Date:         now,
		CurrencyCode: "USD",
		CurrencyName: "Доллар США",
		Nominal:      1,
		Value:        90.5,
		Previous:     89.0,
		CreatedAt:    now,
	}

	if r.CurrencyCode != "USD" {
		t.Errorf("expected USD, got %s", r.CurrencyCode)
	}
	if r.Value != 90.5 {
		t.Errorf("expected Value=90.5, got %f", r.Value)
	}
	if r.Nominal != 1 {
		t.Errorf("expected Nominal=1, got %d", r.Nominal)
	}
}

func TestCryptoRate_fields(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	r := CryptoRate{
		Timestamp: ts,
		Symbol:    "BTCUSDT",
		Open:      40000,
		High:      42000,
		Low:       39000,
		Close:     41000,
		Volume:    1.5,
		PriceRUB:  41000 * 90,
		CreatedAt: time.Now(),
	}

	if r.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", r.Symbol)
	}
	if r.PriceRUB != 3690000 {
		t.Errorf("expected PriceRUB=3690000, got %f", r.PriceRUB)
	}
	if !r.Timestamp.Equal(ts) {
		t.Errorf("timestamp mismatch")
	}
}

func TestCurrencyRate_changeDirection(t *testing.T) {
	tests := []struct {
		value    float64
		previous float64
		wantUp   bool
	}{
		{91.0, 90.0, true},
		{89.0, 90.0, false},
		{90.0, 90.0, false},
	}
	for _, tc := range tests {
		r := CurrencyRate{Value: tc.value, Previous: tc.previous}
		up := r.Value > r.Previous
		if up != tc.wantUp {
			t.Errorf("value=%f previous=%f: expected up=%v, got %v", tc.value, tc.previous, tc.wantUp, up)
		}
	}
}
