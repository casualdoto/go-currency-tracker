package storage

import "time"

// CurrencyRate represents a CBR fiat currency rate stored in PostgreSQL.
type CurrencyRate struct {
	ID           int
	Date         time.Time
	CurrencyCode string
	CurrencyName string
	Nominal      int
	Value        float64
	Previous     float64
	CreatedAt    time.Time
}

// CryptoRate represents a Binance crypto rate stored in ClickHouse.
type CryptoRate struct {
	Timestamp time.Time
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	PriceRUB  float64
	CreatedAt time.Time
}
