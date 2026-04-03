package cryptobackfill

import (
	"sync"
	"time"
)

const cbrPrefetchWorkers = 8

// prefetchCBRUSD loads USD/RUB for distinct calendar days in parallel (bounded),
// so we do not chain one 20s HTTP call after another when USDTRUB is missing.
func (c *Client) prefetchCBRUSD(days []time.Time) map[string]float64 {
	out := make(map[string]float64)
	if c == nil || c.cbr == nil || len(days) == 0 {
		return out
	}
	sem := make(chan struct{}, cbrPrefetchWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range days {
		d := utcDate(d)
		key := d.Format("2006-01-02")
		wg.Add(1)
		sem <- struct{}{}
		go func(day time.Time, k string) {
			defer wg.Done()
			defer func() { <-sem }()
			rub, err := c.usdRubFromCBR(day)
			if err != nil {
				return
			}
			mu.Lock()
			out[k] = rub
			mu.Unlock()
		}(d, key)
	}
	wg.Wait()
	return out
}

func uniqueCalendarDaysFromOpenMs(ms []int64) []time.Time {
	seen := make(map[string]time.Time)
	for _, m := range ms {
		d := utcDate(time.UnixMilli(m).UTC())
		seen[d.Format("2006-01-02")] = d
	}
	out := make([]time.Time, 0, len(seen))
	for _, d := range seen {
		out = append(out, d)
	}
	return out
}
