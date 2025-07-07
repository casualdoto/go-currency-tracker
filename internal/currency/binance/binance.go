// Package binance provides functionality to interact with Binance API
package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
)

// KlineInterval represents the interval for kline/candlestick data
type KlineInterval string

const (
	Interval1m  KlineInterval = "1m"
	Interval5m  KlineInterval = "5m"
	Interval15m KlineInterval = "15m"
	Interval30m KlineInterval = "30m"
	Interval1h  KlineInterval = "1h"
	Interval4h  KlineInterval = "4h"
	Interval1d  KlineInterval = "1d"
	Interval1w  KlineInterval = "1w"
	Interval1M  KlineInterval = "1M"
)

// CryptoRate represents a single cryptocurrency rate data point
type CryptoRate struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// Client represents a Binance API client
type Client struct {
	client *binance.Client
}

// NewClient creates a new Binance API client
func NewClient() *Client {
	// Initialize with empty API keys as we're only using public endpoints
	client := binance.NewClient("", "")
	return &Client{
		client: client,
	}
}

// GetHistoricalKlines retrieves historical kline/candlestick data for a symbol
func (c *Client) GetHistoricalKlines(symbol string, interval KlineInterval, startTime, endTime time.Time) ([]CryptoRate, error) {
	// Convert time to milliseconds for Binance API
	startTimeMs := startTime.UnixMilli()
	endTimeMs := endTime.UnixMilli()

	// Make API call
	klines, err := c.client.NewKlinesService().
		Symbol(symbol).
		Interval(string(interval)).
		StartTime(startTimeMs).
		EndTime(endTimeMs).
		Limit(1000). // Maximum allowed by Binance API
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get historical klines: %w", err)
	}

	// Parse response
	rates := make([]CryptoRate, 0, len(klines))
	for _, kline := range klines {
		// Convert timestamp from milliseconds to time.Time
		timestamp := time.Unix(0, kline.OpenTime*int64(time.Millisecond))

		// Parse price values
		open, _ := strconv.ParseFloat(kline.Open, 64)
		high, _ := strconv.ParseFloat(kline.High, 64)
		low, _ := strconv.ParseFloat(kline.Low, 64)
		close, _ := strconv.ParseFloat(kline.Close, 64)
		volume, _ := strconv.ParseFloat(kline.Volume, 64)

		rates = append(rates, CryptoRate{
			Symbol:    symbol,
			Timestamp: timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		})
	}

	return rates, nil
}

// GetCryptoToRubRate calculates the rate of a cryptocurrency to RUB
// by multiplying the cryptocurrency/USDT rate with the USDT/RUB rate
func (c *Client) GetCryptoToRubRate(cryptoSymbol string, timestamp time.Time) (*CryptoRate, error) {
	// Define time window (1 hour before and after the requested timestamp)
	startTime := timestamp.Add(-1 * time.Hour)
	endTime := timestamp.Add(1 * time.Hour)

	// Get crypto/USDT rate
	cryptoUsdtRates, err := c.GetHistoricalKlines(cryptoSymbol+"USDT", Interval1h, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s/USDT rate: %w", cryptoSymbol, err)
	}
	if len(cryptoUsdtRates) == 0 {
		return nil, fmt.Errorf("no %s/USDT rate data available", cryptoSymbol)
	}

	// Get USDT/RUB rate
	usdtRubRates, err := c.GetHistoricalKlines("USDTRUB", Interval1h, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get USDT/RUB rate: %w", err)
	}
	if len(usdtRubRates) == 0 {
		return nil, fmt.Errorf("no USDT/RUB rate data available")
	}

	// Find closest rates to the requested timestamp
	var closestCryptoRate, closestUsdtRate CryptoRate
	var minCryptoDiff, minUsdtDiff time.Duration = time.Hour * 24, time.Hour * 24

	for _, rate := range cryptoUsdtRates {
		diff := absDuration(rate.Timestamp.Sub(timestamp))
		if diff < minCryptoDiff {
			minCryptoDiff = diff
			closestCryptoRate = rate
		}
	}

	for _, rate := range usdtRubRates {
		diff := absDuration(rate.Timestamp.Sub(timestamp))
		if diff < minUsdtDiff {
			minUsdtDiff = diff
			closestUsdtRate = rate
		}
	}

	// Calculate crypto/RUB rate
	cryptoRubRate := CryptoRate{
		Symbol:    cryptoSymbol + "/RUB",
		Timestamp: timestamp,
		Open:      closestCryptoRate.Open * closestUsdtRate.Open,
		High:      closestCryptoRate.High * closestUsdtRate.High,
		Low:       closestCryptoRate.Low * closestUsdtRate.Low,
		Close:     closestCryptoRate.Close * closestUsdtRate.Close,
		Volume:    closestCryptoRate.Volume,
	}

	return &cryptoRubRate, nil
}

// GetHistoricalCryptoToRubRates retrieves historical cryptocurrency to RUB rates for a date range
func (c *Client) GetHistoricalCryptoToRubRates(cryptoSymbol string, interval KlineInterval, startTime, endTime time.Time) ([]CryptoRate, error) {
	fmt.Printf("GetHistoricalCryptoToRubRates: Getting rates for %s from %s to %s with interval %s\n",
		cryptoSymbol,
		startTime.Format("2006-01-02"),
		endTime.Format("2006-01-02"),
		string(interval))

	// Get crypto/USDT rates
	cryptoUsdtRates, err := c.GetHistoricalKlines(cryptoSymbol+"USDT", interval, startTime, endTime)
	if err != nil {
		fmt.Printf("Error getting %s/USDT rates: %v\n", cryptoSymbol, err)
		return nil, fmt.Errorf("failed to get %s/USDT rates: %w", cryptoSymbol, err)
	}
	if len(cryptoUsdtRates) == 0 {
		fmt.Printf("No %s/USDT rate data available\n", cryptoSymbol)
		return nil, fmt.Errorf("no %s/USDT rate data available", cryptoSymbol)
	}

	fmt.Printf("Got %d %s/USDT rates\n", len(cryptoUsdtRates), cryptoSymbol)

	// Get USDT/RUB rates
	usdtRubRates, err := c.GetHistoricalKlines("USDTRUB", interval, startTime, endTime)
	if err != nil {
		fmt.Printf("Error getting USDT/RUB rates: %v\n", err)
		return nil, fmt.Errorf("failed to get USDT/RUB rates: %w", err)
	}
	if len(usdtRubRates) == 0 {
		fmt.Printf("No USDT/RUB rate data available\n")
		return nil, fmt.Errorf("no USDT/RUB rate data available")
	}

	fmt.Printf("Got %d USDT/RUB rates\n", len(usdtRubRates))

	// Map USDT/RUB rates by timestamp for quick lookup
	usdtRubRatesByTime := make(map[int64]CryptoRate, len(usdtRubRates))
	for _, rate := range usdtRubRates {
		usdtRubRatesByTime[rate.Timestamp.Unix()] = rate
	}

	// Calculate crypto/RUB rates
	result := make([]CryptoRate, 0, len(cryptoUsdtRates))
	matchedCount := 0
	closestCount := 0
	skippedCount := 0

	for _, cryptoRate := range cryptoUsdtRates {
		// Find matching USDT/RUB rate by timestamp
		usdtRate, exists := usdtRubRatesByTime[cryptoRate.Timestamp.Unix()]
		if !exists {
			// Try to find the closest USDT/RUB rate
			var closestUsdtRate CryptoRate
			var minDiff int64 = 24 * 60 * 60 // 24 hours in seconds

			for _, rate := range usdtRubRates {
				diff := abs(rate.Timestamp.Unix() - cryptoRate.Timestamp.Unix())
				if diff < minDiff {
					minDiff = diff
					closestUsdtRate = rate
				}
			}

			// Skip if no close match found (more than 1 hour difference)
			if minDiff > 3600 {
				skippedCount++
				continue
			}

			usdtRate = closestUsdtRate
			closestCount++
		} else {
			matchedCount++
		}

		// Calculate crypto/RUB rate
		cryptoRubRate := CryptoRate{
			Symbol:    cryptoSymbol + "/RUB",
			Timestamp: cryptoRate.Timestamp,
			Open:      cryptoRate.Open * usdtRate.Open,
			High:      cryptoRate.High * usdtRate.High,
			Low:       cryptoRate.Low * usdtRate.Low,
			Close:     cryptoRate.Close * usdtRate.Close,
			Volume:    cryptoRate.Volume,
		}

		result = append(result, cryptoRubRate)
	}

	fmt.Printf("Generated %d %s/RUB rates (exact matches: %d, closest matches: %d, skipped: %d)\n",
		len(result), cryptoSymbol, matchedCount, closestCount, skippedCount)

	if len(result) > 0 {
		fmt.Printf("First result: Symbol=%s, Timestamp=%s\n",
			result[0].Symbol,
			result[0].Timestamp.Format("2006-01-02 15:04:05"))
	}

	return result, nil
}

// Helper function to get absolute value of time.Duration
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// Helper function to get absolute value of int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
