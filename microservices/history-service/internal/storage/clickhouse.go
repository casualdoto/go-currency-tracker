package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseConfig struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

type ClickHouseDB struct {
	conn driver.Conn
}

func NewClickHouseDB(cfg ClickHouseConfig) (*ClickHouseDB, error) {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	for i := 0; i < 10; i++ {
		if err = conn.Ping(context.Background()); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to clickhouse: %w", err)
	}
	return &ClickHouseDB{conn: conn}, nil
}

func (c *ClickHouseDB) Close() error { return c.conn.Close() }

func (c *ClickHouseDB) InitSchema() error {
	return c.conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS crypto_rates (
			timestamp  DateTime,
			symbol     String,
			open       Float64,
			high       Float64,
			low        Float64,
			close      Float64,
			volume     Float64,
			price_rub  Float64,
			created_at DateTime DEFAULT now()
		) ENGINE = ReplacingMergeTree(created_at)
		ORDER BY (symbol, timestamp)
	`)
}

func (c *ClickHouseDB) SaveCryptoRates(rates []CryptoRate) error {
	ctx := context.Background()
	batch, err := c.conn.PrepareBatch(ctx,
		"INSERT INTO crypto_rates (timestamp, symbol, open, high, low, close, volume, price_rub)")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, r := range rates {
		if err := batch.Append(r.Timestamp, r.Symbol, r.Open, r.High, r.Low, r.Close, r.Volume, r.PriceRUB); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (c *ClickHouseDB) GetCryptoRatesBySymbol(symbol string, limit int) ([]CryptoRate, error) {
	rows, err := c.conn.Query(context.Background(), `
		SELECT timestamp, symbol, open, high, low, close, volume, price_rub, created_at
		FROM crypto_rates
		WHERE symbol = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, symbol, uint64(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClickHouseCryptoRates(rows)
}

func (c *ClickHouseDB) GetCryptoRatesByDateRange(symbol string, start, end time.Time) ([]CryptoRate, error) {
	rows, err := c.conn.Query(context.Background(), `
		SELECT timestamp, symbol, open, high, low, close, volume, price_rub, created_at
		FROM crypto_rates
		WHERE symbol = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
	`, symbol, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClickHouseCryptoRates(rows)
}

func (c *ClickHouseDB) GetAvailableCryptoSymbols() ([]string, error) {
	rows, err := c.conn.Query(context.Background(),
		`SELECT DISTINCT symbol FROM crypto_rates ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var symbols []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		symbols = append(symbols, s)
	}
	return symbols, rows.Err()
}

func scanClickHouseCryptoRates(rows driver.Rows) ([]CryptoRate, error) {
	var rates []CryptoRate
	for rows.Next() {
		var r CryptoRate
		if err := rows.Scan(&r.Timestamp, &r.Symbol, &r.Open, &r.High, &r.Low, &r.Close, &r.Volume, &r.PriceRUB, &r.CreatedAt); err != nil {
			return nil, err
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}
