// Package events defines the Kafka message types shared across microservices.
package events

import "time"

// TopicRawRates is the Kafka topic for raw (unprocessed) currency rates
// published by Data Collector Service.
const TopicRawRates = "raw-rates"

// TopicNormalizedRates is the Kafka topic for normalized rates
// published by Normalization Service and consumed by History and Notification services.
const TopicNormalizedRates = "normalized-rates"

// SourceType identifies the origin of a rate event.
type SourceType string

const (
	SourceCBR     SourceType = "cbr"
	SourceBinance SourceType = "binance"
)

// RawCBRRate is a raw currency rate event from CBR API.
type RawCBRRate struct {
	Date        string  `json:"date"`
	CharCode    string  `json:"char_code"`
	NumCode     string  `json:"num_code"`
	Nominal     int     `json:"nominal"`
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	Previous    float64 `json:"previous"`
	CollectedAt time.Time `json:"collected_at"`
}

// RawCBRRatesEvent wraps a batch of CBR rates for Kafka.
type RawCBRRatesEvent struct {
	Source SourceType   `json:"source"`
	Rates  []RawCBRRate `json:"rates"`
}

// RawCryptoRate is a raw OHLCV record from Binance.
type RawCryptoRate struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	CollectedAt time.Time `json:"collected_at"`
}

// RawCryptoRatesEvent wraps a batch of Binance OHLCV records for Kafka.
type RawCryptoRatesEvent struct {
	Source SourceType      `json:"source"`
	Rates  []RawCryptoRate `json:"rates"`
}

// NormalizedCBRRate is a CBR rate normalized to a unified schema.
type NormalizedCBRRate struct {
	Date         time.Time `json:"date"`
	CurrencyCode string    `json:"currency_code"`
	CurrencyName string    `json:"currency_name"`
	Nominal      int       `json:"nominal"`
	ValueRUB     float64   `json:"value_rub"`
	PreviousRUB  float64   `json:"previous_rub"`
}

// NormalizedCBRRatesEvent wraps a batch of normalized CBR rates for Kafka.
type NormalizedCBRRatesEvent struct {
	Source SourceType          `json:"source"`
	Rates  []NormalizedCBRRate `json:"rates"`
}

// NormalizedCryptoRate is a crypto rate normalized and converted to RUB.
type NormalizedCryptoRate struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	PriceRUB  float64   `json:"price_rub"` // Close price in RUB
}

// NormalizedCryptoRatesEvent wraps a batch of normalized crypto rates for Kafka.
type NormalizedCryptoRatesEvent struct {
	Source SourceType             `json:"source"`
	Rates  []NormalizedCryptoRate `json:"rates"`
}
