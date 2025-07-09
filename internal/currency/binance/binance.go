// Package binance provides functionality to interact with Binance API
package binance

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	cbr "github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
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
	// Create a custom HTTP client with increased timeouts
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       60 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	// Initialize with empty API keys as we're only using public endpoints
	client := binance.NewClient("", "")

	// Set the custom HTTP client
	client.HTTPClient = httpClient

	return &Client{
		client: client,
	}
}

// GetHistoricalKlines retrieves historical kline/candlestick data for a symbol
func (c *Client) GetHistoricalKlines(symbol string, interval KlineInterval, startTime, endTime time.Time) ([]CryptoRate, error) {
	// Convert time to milliseconds for Binance API
	startTimeMs := startTime.UnixMilli()
	endTimeMs := endTime.UnixMilli()

	// Retry logic for network issues
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Make API call
		klines, err := c.client.NewKlinesService().
			Symbol(symbol).
			Interval(string(interval)).
			StartTime(startTimeMs).
			EndTime(endTimeMs).
			Limit(1000). // Maximum allowed by Binance API
			Do(context.Background())
		if err != nil {
			lastErr = err
			fmt.Printf("Attempt %d failed for %s: %v\n", attempt+1, symbol, err)

			// Wait before retry (exponential backoff)
			if attempt < maxRetries-1 {
				waitTime := time.Duration(1<<attempt) * time.Second
				fmt.Printf("Waiting %v before retry...\n", waitTime)
				time.Sleep(waitTime)
				continue
			}
			return nil, fmt.Errorf("failed to get historical klines after %d attempts: %w", maxRetries, lastErr)
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

	return nil, fmt.Errorf("failed to get historical klines: %w", lastErr)
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
	if err != nil || len(usdtRubRates) == 0 {
		// If failed to get USDT/RUB rate from Binance, use USD rate from CBR
		fmt.Printf("Error getting USDT/RUB rate or no data available: %v\n", err)
		fmt.Printf("Falling back to CBR USD rate\n")

		// Get USD rate from CBR for the current date
		dateStr := timestamp.Format("2006-01-02")
		usdRate, err := cbr.GetCurrencyRate("USD", dateStr)
		if err != nil || usdRate == nil {
			return nil, fmt.Errorf("failed to get USD/RUB rate from CBR: %w", err)
		}

		fmt.Printf("Got USD rate from CBR for %s: %.4f RUB\n", dateStr, usdRate.Value)

		// Find closest crypto rate to the requested timestamp
		var closestCryptoRate CryptoRate
		var minCryptoDiff time.Duration = time.Hour * 24

		for _, rate := range cryptoUsdtRates {
			diff := absDuration(rate.Timestamp.Sub(timestamp))
			if diff < minCryptoDiff {
				minCryptoDiff = diff
				closestCryptoRate = rate
			}
		}

		// Calculate crypto/RUB rate using CBR USD rate
		cryptoRubRate := CryptoRate{
			Symbol:    cryptoSymbol + "/RUB",
			Timestamp: timestamp,
			Open:      closestCryptoRate.Open * usdRate.Value,
			High:      closestCryptoRate.High * usdRate.Value,
			Low:       closestCryptoRate.Low * usdRate.Value,
			Close:     closestCryptoRate.Close * usdRate.Value,
			Volume:    closestCryptoRate.Volume,
		}

		return &cryptoRubRate, nil
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
	if err != nil || len(usdtRubRates) == 0 {
		// If failed to get USDT/RUB rates from Binance, use USD rates from CBR
		fmt.Printf("Error getting USDT/RUB rates or no data available: %v\n", err)
		fmt.Printf("Falling back to CBR USD rates\n")

		// Create an empty map to store USD rates by date
		cbrRates := make(map[string]float64)

		// Get USD rates for each day in the range
		currentDate := startTime
		for currentDate.Before(endTime) || currentDate.Equal(endTime) {
			dateStr := currentDate.Format("2006-01-02")

			// Get USD rate from CBR
			usdRate, err := cbr.GetCurrencyRate("USD", dateStr)
			if err == nil && usdRate != nil {
				// Save rate to the map by date
				cbrRates[dateStr] = usdRate.Value
				fmt.Printf("Got USD rate from CBR for %s: %.4f RUB\n", dateStr, usdRate.Value)
			} else {
				fmt.Printf("Failed to get USD rate from CBR for %s: %v\n", dateStr, err)
			}

			// Move to the next day
			currentDate = currentDate.AddDate(0, 0, 1)
		}

		// If couldn't get any rates from CBR, return an error
		if len(cbrRates) == 0 {
			return nil, fmt.Errorf("failed to get USD/RUB rates from CBR and USDT/RUB rates from Binance")
		}

		// Calculate crypto/RUB rates using USD rates from CBR
		result := make([]CryptoRate, 0, len(cryptoUsdtRates))

		for _, cryptoRate := range cryptoUsdtRates {
			// Get date as string
			dateStr := cryptoRate.Timestamp.Format("2006-01-02")

			// Find USD rate for this date
			usdRate, exists := cbrRates[dateStr]
			if !exists {
				// If no rate for this date, find the closest one
				var closestDate string
				var minDiff int = 100 // maximum difference in days

				for date := range cbrRates {
					rateDate, _ := time.Parse("2006-01-02", date)
					diff := int(abs(rateDate.Unix()-cryptoRate.Timestamp.Unix()) / (24 * 60 * 60))
					if diff < minDiff {
						minDiff = diff
						closestDate = date
					}
				}

				// If found closest date within 7 days, use it
				if minDiff <= 7 && closestDate != "" {
					usdRate = cbrRates[closestDate]
					fmt.Printf("Using closest USD rate from %s for %s\n", closestDate, dateStr)
				} else {
					// Otherwise skip this data point
					fmt.Printf("Skipping rate for %s, no close USD rate found\n", dateStr)
					continue
				}
			}

			// Calculate crypto/RUB rate
			cryptoRubRate := CryptoRate{
				Symbol:    cryptoSymbol + "/RUB",
				Timestamp: cryptoRate.Timestamp,
				Open:      cryptoRate.Open * usdRate,
				High:      cryptoRate.High * usdRate,
				Low:       cryptoRate.Low * usdRate,
				Close:     cryptoRate.Close * usdRate,
				Volume:    cryptoRate.Volume,
			}

			result = append(result, cryptoRubRate)
		}

		fmt.Printf("Generated %d %s/RUB rates using CBR USD rates\n", len(result), cryptoSymbol)

		if len(result) > 0 {
			fmt.Printf("First result: Symbol=%s, Timestamp=%s\n",
				result[0].Symbol,
				result[0].Timestamp.Format("2006-01-02 15:04:05"))
		}

		return result, nil
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

// GetCurrentPrice gets current price for a symbol (24hr ticker)
func (c *Client) GetCurrentPrice(symbol string) (*CryptoRate, error) {
	// Retry logic for network issues
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get 24hr ticker statistics
		ticker, err := c.client.NewListPriceChangeStatsService().
			Symbol(symbol).
			Do(context.Background())
		if err != nil {
			lastErr = err
			fmt.Printf("Attempt %d failed for %s ticker: %v\n", attempt+1, symbol, err)

			// Wait before retry (exponential backoff)
			if attempt < maxRetries-1 {
				waitTime := time.Duration(1<<attempt) * time.Second
				fmt.Printf("Waiting %v before retry...\n", waitTime)
				time.Sleep(waitTime)
				continue
			}
			return nil, fmt.Errorf("failed to get current price after %d attempts: %w", maxRetries, lastErr)
		}

		if len(ticker) == 0 {
			return nil, fmt.Errorf("no ticker data available for %s", symbol)
		}

		// Parse the first ticker result
		t := ticker[0]

		openPrice, _ := strconv.ParseFloat(t.OpenPrice, 64)
		highPrice, _ := strconv.ParseFloat(t.HighPrice, 64)
		lowPrice, _ := strconv.ParseFloat(t.LowPrice, 64)
		lastPrice, _ := strconv.ParseFloat(t.LastPrice, 64)
		volume, _ := strconv.ParseFloat(t.Volume, 64)

		result := &CryptoRate{
			Symbol:    symbol,
			Timestamp: time.Now(),
			Open:      openPrice,
			High:      highPrice,
			Low:       lowPrice,
			Close:     lastPrice,
			Volume:    volume,
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed to get current price: %w", lastErr)
}

// GetCurrentCryptoToRubRate gets current cryptocurrency to RUB rate
func (c *Client) GetCurrentCryptoToRubRate(cryptoSymbol string) (*CryptoRate, error) {
	// Get crypto/USDT current price
	cryptoUsdtRate, err := c.GetCurrentPrice(cryptoSymbol + "USDT")
	if err != nil {
		return nil, fmt.Errorf("failed to get %s/USDT current price: %w", cryptoSymbol, err)
	}

	// Get current USD rate from CBR (USDT ≈ USD for conversion)
	usdRate, err := cbr.GetCurrencyRate("USD", "")
	if err != nil || usdRate == nil {
		return nil, fmt.Errorf("failed to get USD/RUB rate from CBR: %w", err)
	}

	// Check if USD rate is valid (not zero)
	if usdRate.Value == 0 {
		return nil, fmt.Errorf("USD rate from CBR is zero")
	}

	// Calculate crypto/RUB rate using CBR USD rate
	cryptoRubRate := CryptoRate{
		Symbol:    cryptoSymbol + "/RUB",
		Timestamp: time.Now(),
		Open:      cryptoUsdtRate.Open * usdRate.Value,
		High:      cryptoUsdtRate.High * usdRate.Value,
		Low:       cryptoUsdtRate.Low * usdRate.Value,
		Close:     cryptoUsdtRate.Close * usdRate.Value,
		Volume:    cryptoUsdtRate.Volume,
	}

	fmt.Printf("Calculated %s/RUB rate: %.2f RUB (%.2f USDT * %.4f RUB/USD)\n",
		cryptoSymbol, cryptoRubRate.Close, cryptoUsdtRate.Close, usdRate.Value)

	return &cryptoRubRate, nil
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
