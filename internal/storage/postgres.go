// Package storage provides database storage functionality
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConfig contains database connection configuration
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// PostgresDB represents a PostgreSQL database connection
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(cfg PostgresConfig) (*PostgresDB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// InitSchema initializes the database schema
func (p *PostgresDB) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS currency_rates (
		id SERIAL PRIMARY KEY,
		date DATE NOT NULL,
		currency_code VARCHAR(3) NOT NULL,
		currency_name VARCHAR(100) NOT NULL,
		nominal INTEGER NOT NULL,
		value DECIMAL(12, 4) NOT NULL,
		previous DECIMAL(12, 4),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(date, currency_code)
	);
	
	CREATE INDEX IF NOT EXISTS idx_currency_rates_date ON currency_rates(date);
	CREATE INDEX IF NOT EXISTS idx_currency_rates_code ON currency_rates(currency_code);
	`

	_, err := p.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// CurrencyRate represents a currency rate record in the database
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

// SaveCurrencyRates saves multiple currency rates to the database
func (p *PostgresDB) SaveCurrencyRates(rates []CurrencyRate) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO currency_rates (date, currency_code, currency_name, nominal, value, previous)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (date, currency_code) 
		DO UPDATE SET 
			currency_name = EXCLUDED.currency_name,
			nominal = EXCLUDED.nominal,
			value = EXCLUDED.value,
			previous = EXCLUDED.previous,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, rate := range rates {
		_, err := stmt.Exec(
			rate.Date,
			rate.CurrencyCode,
			rate.CurrencyName,
			rate.Nominal,
			rate.Value,
			rate.Previous,
		)
		if err != nil {
			return fmt.Errorf("failed to insert currency rate: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetCurrencyRatesByDate retrieves currency rates for a specific date
func (p *PostgresDB) GetCurrencyRatesByDate(date time.Time) ([]CurrencyRate, error) {
	rows, err := p.db.Query(`
		SELECT id, date, currency_code, currency_name, nominal, value, previous, created_at
		FROM currency_rates
		WHERE date = $1
		ORDER BY currency_code
	`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query currency rates: %w", err)
	}
	defer rows.Close()

	var rates []CurrencyRate
	for rows.Next() {
		var rate CurrencyRate
		if err := rows.Scan(
			&rate.ID,
			&rate.Date,
			&rate.CurrencyCode,
			&rate.CurrencyName,
			&rate.Nominal,
			&rate.Value,
			&rate.Previous,
			&rate.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan currency rate: %w", err)
		}
		rates = append(rates, rate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over currency rates: %w", err)
	}

	return rates, nil
}

// GetCurrencyRate retrieves a specific currency rate for a date
func (p *PostgresDB) GetCurrencyRate(code string, date time.Time) (*CurrencyRate, error) {
	var rate CurrencyRate
	err := p.db.QueryRow(`
		SELECT id, date, currency_code, currency_name, nominal, value, previous, created_at
		FROM currency_rates
		WHERE currency_code = $1 AND date = $2
	`, code, date).Scan(
		&rate.ID,
		&rate.Date,
		&rate.CurrencyCode,
		&rate.CurrencyName,
		&rate.Nominal,
		&rate.Value,
		&rate.Previous,
		&rate.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("currency rate not found")
		}
		return nil, fmt.Errorf("failed to query currency rate: %w", err)
	}

	return &rate, nil
}

// GetAvailableDates retrieves a list of dates for which currency rates are available
func (p *PostgresDB) GetAvailableDates() ([]time.Time, error) {
	rows, err := p.db.Query(`
		SELECT DISTINCT date
		FROM currency_rates
		ORDER BY date DESC
		LIMIT 30
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query available dates: %w", err)
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, fmt.Errorf("failed to scan date: %w", err)
		}
		dates = append(dates, date)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over dates: %w", err)
	}

	return dates, nil
}
