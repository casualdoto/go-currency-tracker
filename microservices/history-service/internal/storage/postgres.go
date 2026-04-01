package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type PostgresDB struct {
	db *sql.DB
}

func NewPostgresDB(cfg Config) (*PostgresDB, error) {
	port, _ := strconv.Atoi(cfg.Port)
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to postgres: %w", err)
	}
	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) Close() error { return p.db.Close() }

func (p *PostgresDB) InitSchema() error {
	_, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS cbr_rates (
			id SERIAL PRIMARY KEY,
			date DATE NOT NULL,
			currency_code VARCHAR(3) NOT NULL,
			currency_name VARCHAR(100) NOT NULL,
			nominal INTEGER NOT NULL,
			value DECIMAL(12,4) NOT NULL,
			previous DECIMAL(12,4),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(date, currency_code)
		);
		CREATE INDEX IF NOT EXISTS idx_cbr_rates_date ON cbr_rates(date);
		CREATE INDEX IF NOT EXISTS idx_cbr_rates_code ON cbr_rates(currency_code);

		CREATE TABLE IF NOT EXISTS crypto_rates (
			id SERIAL PRIMARY KEY,
			timestamp BIGINT NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			open DECIMAL(24,8) NOT NULL,
			high DECIMAL(24,8) NOT NULL,
			low DECIMAL(24,8) NOT NULL,
			close DECIMAL(24,8) NOT NULL,
			volume DECIMAL(24,8) NOT NULL,
			price_rub DECIMAL(24,8),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(timestamp, symbol)
		);
		CREATE INDEX IF NOT EXISTS idx_crypto_rates_timestamp ON crypto_rates(timestamp);
		CREATE INDEX IF NOT EXISTS idx_crypto_rates_symbol ON crypto_rates(symbol);
	`)
	return err
}

// CurrencyRate matches the monolith's CurrencyRate struct.
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

func (p *PostgresDB) SaveCurrencyRates(rates []CurrencyRate) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO cbr_rates (date, currency_code, currency_name, nominal, value, previous)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (date, currency_code) DO UPDATE SET
			currency_name = EXCLUDED.currency_name,
			nominal = EXCLUDED.nominal,
			value = EXCLUDED.value,
			previous = EXCLUDED.previous,
			created_at = NOW()
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rates {
		if _, err := stmt.Exec(r.Date, r.CurrencyCode, r.CurrencyName, r.Nominal, r.Value, r.Previous); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *PostgresDB) GetCurrencyRatesByDate(date time.Time) ([]CurrencyRate, error) {
	rows, err := p.db.Query(`
		SELECT id, date, currency_code, currency_name, nominal, value, previous, created_at
		FROM cbr_rates WHERE date = $1 ORDER BY currency_code
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCurrencyRates(rows)
}

func (p *PostgresDB) GetCurrencyRatesByDateRange(code string, start, end time.Time) ([]CurrencyRate, error) {
	rows, err := p.db.Query(`
		SELECT id, date, currency_code, currency_name, nominal, value, previous, created_at
		FROM cbr_rates WHERE currency_code = $1 AND date >= $2 AND date <= $3 ORDER BY date DESC
	`, code, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCurrencyRates(rows)
}

func scanCurrencyRates(rows *sql.Rows) ([]CurrencyRate, error) {
	var rates []CurrencyRate
	for rows.Next() {
		var r CurrencyRate
		if err := rows.Scan(&r.ID, &r.Date, &r.CurrencyCode, &r.CurrencyName, &r.Nominal, &r.Value, &r.Previous, &r.CreatedAt); err != nil {
			return nil, err
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}

// CryptoRate matches the monolith's CryptoRate struct.
type CryptoRate struct {
	ID        int
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

func (p *PostgresDB) SaveCryptoRates(rates []CryptoRate) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO crypto_rates (timestamp, symbol, open, high, low, close, volume, price_rub)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (timestamp, symbol) DO UPDATE SET
			open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
			close = EXCLUDED.close, volume = EXCLUDED.volume, price_rub = EXCLUDED.price_rub,
			created_at = NOW()
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rates {
		if _, err := stmt.Exec(r.Timestamp.Unix(), r.Symbol, r.Open, r.High, r.Low, r.Close, r.Volume, r.PriceRUB); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *PostgresDB) GetCryptoRatesBySymbol(symbol string, limit int) ([]CryptoRate, error) {
	rows, err := p.db.Query(`
		SELECT id, timestamp, symbol, open, high, low, close, volume, COALESCE(price_rub,0), created_at
		FROM crypto_rates WHERE symbol = $1 ORDER BY timestamp DESC LIMIT $2
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCryptoRates(rows)
}

func (p *PostgresDB) GetCryptoRatesByDateRange(symbol string, start, end time.Time) ([]CryptoRate, error) {
	rows, err := p.db.Query(`
		SELECT id, timestamp, symbol, open, high, low, close, volume, COALESCE(price_rub,0), created_at
		FROM crypto_rates WHERE symbol = $1 AND timestamp >= $2 AND timestamp <= $3 ORDER BY timestamp DESC
	`, symbol, start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCryptoRates(rows)
}

func (p *PostgresDB) GetAvailableCryptoSymbols() ([]string, error) {
	rows, err := p.db.Query(`SELECT DISTINCT symbol FROM crypto_rates ORDER BY symbol`)
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

func scanCryptoRates(rows *sql.Rows) ([]CryptoRate, error) {
	var rates []CryptoRate
	for rows.Next() {
		var r CryptoRate
		var ts int64
		if err := rows.Scan(&r.ID, &ts, &r.Symbol, &r.Open, &r.High, &r.Low, &r.Close, &r.Volume, &r.PriceRUB, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Timestamp = time.Unix(ts, 0)
		rates = append(rates, r)
	}
	return rates, rows.Err()
}
