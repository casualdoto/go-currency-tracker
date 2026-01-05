package types

import (
	"time"
)

// CurrencyRate represents a currency exchange rate from CBR
type CurrencyRate struct {
	ID           int       `json:"id" db:"id"`
	Source       string    `json:"source" db:"source"` // 'cbr'
	CurrencyCode string    `json:"currency_code" db:"currency_code"`
	CurrencyName string    `json:"currency_name" db:"currency_name"`
	Rate         float64   `json:"rate" db:"rate"`
	BaseCurrency string    `json:"base_currency" db:"base_currency"`
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// CryptoRate represents a cryptocurrency rate from Binance
type CryptoRate struct {
	ID                 int       `json:"id" db:"id"`
	Symbol             string    `json:"symbol" db:"symbol"`
	BaseAsset          string    `json:"base_asset" db:"base_asset"`
	QuoteAsset         string    `json:"quote_asset" db:"quote_asset"`
	Price              float64   `json:"price" db:"price"`
	Volume             float64   `json:"volume" db:"volume"`
	PriceChangePercent float64   `json:"price_change_percent" db:"price_change_percent"`
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// RateResponse represents the response for rate queries
type RateResponse struct {
	CurrencyRates []CurrencyRate `json:"currency_rates"`
	CryptoRates   []CryptoRate   `json:"crypto_rates"`
	Timestamp     time.Time      `json:"timestamp"`
}

// ErrorResponse represents error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
