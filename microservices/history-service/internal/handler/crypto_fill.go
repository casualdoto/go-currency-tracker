package handler

import (
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

const maxCryptoAutoBackfillSpanDays = 400

func inclusiveCalendarDaysUTC(from, to time.Time) int {
	fy, fm, fd := from.UTC().Date()
	ty, tm, td := to.UTC().Date()
	fromD := time.Date(fy, fm, fd, 0, 0, 0, 0, time.UTC)
	toD := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	if toD.Before(fromD) {
		return 0
	}
	return int(toD.Sub(fromD).Hours()/24) + 1
}

func distinctUTCDayCount(rates []storage.CryptoRate) int {
	if len(rates) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(rates))
	for _, r := range rates {
		t := r.Timestamp.UTC()
		key := t.Format("2006-01-02")
		seen[key] = struct{}{}
	}
	return len(seen)
}

// backfillCryptoRange loads daily Binance klines into ClickHouse when the stored series
// does not cover all calendar days in [from, to] (same idea as CBR archive fill).
func (h *Handler) backfillCryptoRange(symbol string, from, to time.Time, rates []storage.CryptoRate) bool {
	if h.crypto == nil {
		return false
	}
	span := inclusiveCalendarDaysUTC(from, to)
	if span <= 0 {
		return false
	}
	if span > maxCryptoAutoBackfillSpanDays {
		log.Printf("crypto backfill: range too long (%d days), skipping auto-fetch", span)
		return false
	}
	have := distinctUTCDayCount(rates)
	if have >= span && len(rates) > 0 {
		return false
	}

	rows, err := h.crypto.FetchDailyRUBRates(symbol, from, to)
	if err != nil {
		log.Printf("crypto backfill: fetch failed: %v", err)
		return false
	}
	if len(rows) == 0 {
		return false
	}
	if err := h.ch.SaveCryptoRates(rows); err != nil {
		log.Printf("crypto backfill: save clickhouse: %v", err)
		return false
	}
	log.Printf("crypto backfill: stored %d daily rows for %s", len(rows), symbol)
	return true
}
