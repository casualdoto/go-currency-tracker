// Package cryptobackfill loads missing daily crypto history from Binance public klines
// (symbol USDT + USDTRUB close, or CBR USD/RUB when USDTRUB is unavailable).
package cryptobackfill

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/cbrbackfill"
	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
)

const (
	defaultBinanceBase = "https://api.binance.com"
	interval1d         = "1d"
	usdtRubSymbol      = "USDTRUB"
	// pause between Binance HTTP calls to reduce rate-limit risk
	binanceRequestPause = 120 * time.Millisecond
)

// Client fetches Binance klines and optionally uses CBR for USD/RUB fallback.
type Client struct {
	binanceBase string
	http        *http.Client
	cbr         *cbrbackfill.Client
}

// New returns a client. binanceBase may be empty to use the public API default.
func New(binanceBase string, cbr *cbrbackfill.Client) *Client {
	if binanceBase == "" {
		binanceBase = defaultBinanceBase
	}
	return &Client{
		binanceBase: binanceBase,
		http:        &http.Client{Timeout: 25 * time.Second},
		cbr:         cbr,
	}
}

// FetchDailyRUBRates returns one row per UTC calendar day (Binance 1d open time) for symbol (e.g. BTCUSDT).
func (c *Client) FetchDailyRUBRates(symbol string, from, to time.Time) ([]storage.CryptoRate, error) {
	if c == nil {
		return nil, fmt.Errorf("cryptobackfill client is nil")
	}
	from = utcDate(from)
	to = utcDate(to)
	if to.Before(from) {
		return nil, nil
	}

	var cryptoKlines, usdtRub []klineOHLCV
	var errCrypto, errUSDT error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cryptoKlines, errCrypto = c.fetchAllDailyKlines(symbol, from, to)
	}()
	go func() {
		defer wg.Done()
		usdtRub, errUSDT = c.fetchAllDailyKlines(usdtRubSymbol, from, to)
	}()
	wg.Wait()
	if errCrypto != nil {
		return nil, errCrypto
	}
	if errUSDT != nil {
		usdtRub = nil
	}

	rubCloseByOpenMs := make(map[int64]float64, len(usdtRub))
	for _, k := range usdtRub {
		rubCloseByOpenMs[k.openTimeMs] = k.close
	}

	needCBR := make(map[string]time.Time)
	for _, k := range cryptoKlines {
		if r, ok := rubCloseByOpenMs[k.openTimeMs]; ok && r > 0 {
			continue
		}
		day := utcDate(time.UnixMilli(k.openTimeMs).UTC())
		needCBR[day.Format("2006-01-02")] = day
	}
	cbrDays := make([]time.Time, 0, len(needCBR))
	for _, d := range needCBR {
		cbrDays = append(cbrDays, d)
	}
	cbrCache := c.prefetchCBRUSD(cbrDays)

	out := make([]storage.CryptoRate, 0, len(cryptoKlines))
	for _, k := range cryptoKlines {
		rub, ok := rubCloseByOpenMs[k.openTimeMs]
		if !ok || rub <= 0 {
			key := utcDate(time.UnixMilli(k.openTimeMs).UTC()).Format("2006-01-02")
			r, hit := cbrCache[key]
			if !hit || r <= 0 {
				continue
			}
			rub = r
			ok = true
		}
		if !ok || rub <= 0 {
			continue
		}
		ts := time.UnixMilli(k.openTimeMs).UTC()
		out = append(out, storage.CryptoRate{
			Timestamp: ts,
			Symbol:    symbol,
			Open:      k.open * rub,
			High:      k.high * rub,
			Low:       k.low * rub,
			Close:     k.close * rub,
			Volume:    k.volume,
			PriceRUB:  k.close * rub,
		})
	}
	return out, nil
}

// FetchIntervalRUBRates loads klines at the given Binance interval (e.g. 15m, 1h), converts to RUB
// via USDTRUB (same interval) or CBR USD when USDTRUB is missing — mirrors monolith behaviour.
func (c *Client) FetchIntervalRUBRates(symbol, interval string, from, to time.Time) ([]storage.CryptoRate, error) {
	if c == nil {
		return nil, fmt.Errorf("cryptobackfill client is nil")
	}
	fromU := utcDate(from)
	toU := utcDate(to)
	if toU.Before(fromU) {
		return nil, nil
	}
	startMs := fromU.UnixMilli()
	endMs := toU.AddDate(0, 0, 1).UnixMilli()

	var cryptoKlines, usdtKlines []klineOHLCV
	var errCrypto, errUSDT error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cryptoKlines, errCrypto = c.fetchAllKlinesPaginated(symbol, interval, startMs, endMs)
	}()
	go func() {
		defer wg.Done()
		usdtKlines, errUSDT = c.fetchAllKlinesPaginated(usdtRubSymbol, interval, startMs, endMs)
	}()
	wg.Wait()
	if errCrypto != nil {
		return nil, errCrypto
	}
	if len(cryptoKlines) == 0 {
		return nil, fmt.Errorf("no crypto klines for %s %s", symbol, interval)
	}
	if errUSDT != nil {
		usdtKlines = nil
	}

	var out []storage.CryptoRate
	if len(usdtKlines) == 0 {
		out = c.mergeKlinesCBRFallback(cryptoKlines, symbol)
		if len(out) == 0 {
			return nil, fmt.Errorf("no USDT/RUB klines and CBR fallback produced no rows")
		}
		return out, nil
	}
	out = c.mergeKlinesToRUB(cryptoKlines, usdtKlines, symbol)
	if len(out) == 0 {
		return nil, fmt.Errorf("merge crypto+USDTRUB produced no rows")
	}
	return out, nil
}

func (c *Client) mergeKlinesToRUB(crypto, usdt []klineOHLCV, symbol string) []storage.CryptoRate {
	usdtBySec := make(map[int64]klineOHLCV, len(usdt))
	for _, u := range usdt {
		sec := time.UnixMilli(u.openTimeMs).UTC().Unix()
		usdtBySec[sec] = u
	}
	out := make([]storage.CryptoRate, 0, len(crypto))
	for _, k := range crypto {
		sec := time.UnixMilli(k.openTimeMs).UTC().Unix()
		u, ok := usdtBySec[sec]
		if !ok {
			var best klineOHLCV
			minDiff := int64(math.MaxInt64)
			for _, cand := range usdt {
				d := absMilli(cand.openTimeMs - k.openTimeMs)
				if d < minDiff {
					minDiff = d
					best = cand
				}
			}
			// Allow up to 2h skew between BTCUSDT and USDTRUB candles (exchange / clock quirks).
			if minDiff > 2*3600*1000 {
				continue
			}
			u = best
		}
		ts := time.UnixMilli(k.openTimeMs).UTC()
		closeRub := k.close * u.close
		out = append(out, storage.CryptoRate{
			Timestamp: ts,
			Symbol:    symbol,
			Open:      k.open * u.open,
			High:      k.high * u.high,
			Low:       k.low * u.low,
			Close:     closeRub,
			Volume:    k.volume,
			PriceRUB:  closeRub,
		})
	}
	return out
}

func (c *Client) mergeKlinesCBRFallback(crypto []klineOHLCV, symbol string) []storage.CryptoRate {
	if c.cbr == nil {
		return nil
	}
	keys := make([]int64, len(crypto))
	for i, k := range crypto {
		keys[i] = k.openTimeMs
	}
	days := uniqueCalendarDaysFromOpenMs(keys)
	cbrCache := c.prefetchCBRUSD(days)
	out := make([]storage.CryptoRate, 0, len(crypto))
	for _, k := range crypto {
		key := utcDate(time.UnixMilli(k.openTimeMs).UTC()).Format("2006-01-02")
		rub, hit := cbrCache[key]
		if !hit || rub <= 0 {
			continue
		}
		ts := time.UnixMilli(k.openTimeMs).UTC()
		closeRub := k.close * rub
		out = append(out, storage.CryptoRate{
			Timestamp: ts,
			Symbol:    symbol,
			Open:      k.open * rub,
			High:      k.high * rub,
			Low:       k.low * rub,
			Close:     closeRub,
			Volume:    k.volume,
			PriceRUB:  closeRub,
		})
	}
	return out
}

func absMilli(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (c *Client) fetchAllKlinesPaginated(symbol, interval string, startMs, endMs int64) ([]klineOHLCV, error) {
	var all []klineOHLCV
	cur := startMs
	for cur < endMs {
		chunk, err := c.fetchKlinesPage(symbol, interval, cur, endMs)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		all = append(all, chunk...)
		last := chunk[len(chunk)-1].openTimeMs
		next := last + 1
		if next <= cur {
			break
		}
		cur = next
		if len(chunk) < 1000 {
			break
		}
		time.Sleep(binanceRequestPause)
	}
	return all, nil
}

func (c *Client) usdRubFromCBR(day time.Time) (float64, error) {
	if c.cbr == nil {
		return 0, fmt.Errorf("cbr client disabled")
	}
	rates, _, err := c.cbr.FetchDayWithFallback(day)
	if err != nil {
		return 0, err
	}
	for _, r := range rates {
		if r.CurrencyCode == "USD" && r.Nominal > 0 {
			return r.Value / float64(r.Nominal), nil
		}
	}
	return 0, fmt.Errorf("no USD in CBR for %s", day.Format("2006-01-02"))
}

func utcDate(t time.Time) time.Time {
	t = t.UTC()
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type klineOHLCV struct {
	openTimeMs int64
	open       float64
	high       float64
	low        float64
	close      float64
	volume     float64
}

func (c *Client) fetchAllDailyKlines(symbol string, from, to time.Time) ([]klineOHLCV, error) {
	var all []klineOHLCV
	cur := from
	endInclusive := to
	for !cur.After(endInclusive) {
		chunkEnd := cur.AddDate(0, 0, 999)
		if chunkEnd.After(endInclusive) {
			chunkEnd = endInclusive
		}
		startMs := cur.UnixMilli()
		endMs := chunkEnd.AddDate(0, 0, 1).UnixMilli()
		part, err := c.fetchKlinesPage(symbol, interval1d, startMs, endMs)
		if err != nil {
			return nil, err
		}
		all = append(all, part...)
		time.Sleep(binanceRequestPause)
		cur = chunkEnd.AddDate(0, 0, 1)
	}
	return all, nil
}

func (c *Client) fetchKlinesPage(symbol, interval string, startMs, endMs int64) ([]klineOHLCV, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("startTime", strconv.FormatInt(startMs, 10))
	q.Set("endTime", strconv.FormatInt(endMs, 10))
	q.Set("limit", "1000")
	reqURL := fmt.Sprintf("%s/api/v3/klines?%s", c.binanceBase, q.Encode())

	resp, err := c.http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("binance get: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance klines %s: status %d: %s", symbol, resp.StatusCode, truncate(body, 200))
	}
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance klines decode: %w", err)
	}
	out := make([]klineOHLCV, 0, len(raw))
	for _, row := range raw {
		k, ok := parseKlineRow(row)
		if ok {
			out = append(out, k)
		}
	}
	return out, nil
}

func parseKlineRow(row []interface{}) (klineOHLCV, bool) {
	if len(row) < 6 {
		return klineOHLCV{}, false
	}
	otf, ok := row[0].(float64)
	if !ok {
		return klineOHLCV{}, false
	}
	openTimeMs := int64(otf)
	open, _ := parseFloatField(row[1])
	high, _ := parseFloatField(row[2])
	low, _ := parseFloatField(row[3])
	close, _ := parseFloatField(row[4])
	vol, _ := parseFloatField(row[5])
	return klineOHLCV{
		openTimeMs: openTimeMs,
		open:       open,
		high:       high,
		low:        low,
		close:      close,
		volume:     vol,
	}, true
}

func parseFloatField(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	case float64:
		return x, true
	default:
		return 0, false
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
