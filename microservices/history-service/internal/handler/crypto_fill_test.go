package handler

import (
	"testing"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

func TestInclusiveCalendarDaysUTC(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if n := inclusiveCalendarDaysUTC(from, to); n != 31 {
		t.Fatalf("got %d want 31", n)
	}
	same := time.Date(2026, 4, 2, 15, 0, 0, 0, time.UTC)
	if n := inclusiveCalendarDaysUTC(same, same); n != 1 {
		t.Fatalf("got %d want 1", n)
	}
}

func TestDistinctUTCDayCount(t *testing.T) {
	d1 := time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rates := []storage.CryptoRate{
		{Timestamp: d1},
		{Timestamp: d2},
		{Timestamp: d3},
	}
	if n := distinctUTCDayCount(rates); n != 2 {
		t.Fatalf("got %d want 2", n)
	}
}
