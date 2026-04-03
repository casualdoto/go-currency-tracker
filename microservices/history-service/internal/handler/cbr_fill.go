package handler

import (
	"log"
	"time"
)

const maxCBRAutoBackfillSpanDays = 400

func calendarDateUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// backfillCBRMissingDays loads archive daily_json for each calendar day in [from,to]
// where the requested currency code is missing from PostgreSQL.
// Returns true if at least one day was fetched and stored.
func (h *Handler) backfillCBRMissingDays(code string, from, to time.Time) bool {
	if h.cbr == nil {
		return false
	}
	start := calendarDateUTC(from)
	end := calendarDateUTC(to)
	if end.Before(start) {
		return false
	}
	span := int(end.Sub(start).Hours()/24) + 1
	if span > maxCBRAutoBackfillSpanDays {
		log.Printf("cbr backfill: range too long (%d days), skipping auto-fetch", span)
		return false
	}
	any := false
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		has, err := h.pg.HasCBRRateOnDay(code, d)
		if err != nil {
			log.Printf("cbr backfill: has check %s %s: %v", code, d.Format("2006-01-02"), err)
			continue
		}
		if has {
			continue
		}
		rates, srcDay, err := h.cbr.FetchDayWithFallback(d)
		if err != nil {
			log.Printf("cbr backfill: fetch %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		if !calendarDateUTC(srcDay).Equal(d) {
			log.Printf("cbr backfill: using archive %s for missing day %s", srcDay.Format("2006-01-02"), d.Format("2006-01-02"))
			for i := range rates {
				rates[i].Date = d
			}
		}
		if err := h.pg.SaveCurrencyRates(rates); err != nil {
			log.Printf("cbr backfill: save %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		any = true
		log.Printf("cbr backfill: stored %d CBR rows for %s", len(rates), d.Format("2006-01-02"))
		time.Sleep(120 * time.Millisecond)
	}
	return any
}

// backfillCBRDayIfEmpty loads the full daily sheet when the DB has no rows for that date.
func (h *Handler) backfillCBRDayIfEmpty(day time.Time) {
	if h.cbr == nil {
		return
	}
	d := calendarDateUTC(day)
	rates, srcDay, err := h.cbr.FetchDayWithFallback(d)
	if err != nil {
		log.Printf("cbr backfill: single-day fetch %s: %v", d.Format("2006-01-02"), err)
		return
	}
	if !calendarDateUTC(srcDay).Equal(d) {
		log.Printf("cbr backfill: single-day using archive %s for %s", srcDay.Format("2006-01-02"), d.Format("2006-01-02"))
		for i := range rates {
			rates[i].Date = d
		}
	}
	if err := h.pg.SaveCurrencyRates(rates); err != nil {
		log.Printf("cbr backfill: single-day save %s: %v", d.Format("2006-01-02"), err)
		return
	}
	log.Printf("cbr backfill: stored %d CBR rows for %s (list-by-date)", len(rates), d.Format("2006-01-02"))
}
