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

	CREATE TABLE IF NOT EXISTS crypto_rates (
		id SERIAL PRIMARY KEY,
		timestamp BIGINT NOT NULL,
		symbol VARCHAR(20) NOT NULL,
		open DECIMAL(24, 8) NOT NULL,
		high DECIMAL(24, 8) NOT NULL,
		low DECIMAL(24, 8) NOT NULL,
		close DECIMAL(24, 8) NOT NULL,
		volume DECIMAL(24, 8) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(timestamp, symbol)
	);
	
	CREATE INDEX IF NOT EXISTS idx_crypto_rates_timestamp ON crypto_rates(timestamp);
	CREATE INDEX IF NOT EXISTS idx_crypto_rates_symbol ON crypto_rates(symbol);
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

// GetCurrencyRatesByDateRange retrieves currency rates for a specific currency within a date range
func (p *PostgresDB) GetCurrencyRatesByDateRange(code string, startDate, endDate time.Time) ([]CurrencyRate, error) {
	rows, err := p.db.Query(`
		SELECT id, date, currency_code, currency_name, nominal, value, previous, created_at
		FROM currency_rates
		WHERE currency_code = $1 AND date >= $2 AND date <= $3
		ORDER BY date DESC
	`, code, startDate, endDate)
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

// CryptoRate represents a cryptocurrency rate record in the database
type CryptoRate struct {
	ID        int
	Timestamp time.Time
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CreatedAt time.Time
}

// SaveCryptoRates saves multiple cryptocurrency rates to the database
func (p *PostgresDB) SaveCryptoRates(rates []CryptoRate) error {
	fmt.Printf("SaveCryptoRates: Attempting to save %d rates\n", len(rates))

	if len(rates) > 0 {
		fmt.Printf("First rate: Symbol=%s, Timestamp=%s, Unix=%d\n",
			rates[0].Symbol,
			rates[0].Timestamp.Format("2006-01-02 15:04:05"),
			rates[0].Timestamp.Unix())
	}

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO crypto_rates (timestamp, symbol, open, high, low, close, volume)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (timestamp, symbol) 
		DO UPDATE SET 
			open = EXCLUDED.open,
			high = EXCLUDED.high,
			low = EXCLUDED.low,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume,
			created_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, rate := range rates {
		// Convert time.Time to Unix timestamp in seconds
		unixTimestamp := rate.Timestamp.Unix()

		fmt.Printf("Inserting rate: Symbol=%s, Timestamp=%s, Unix=%d\n",
			rate.Symbol,
			rate.Timestamp.Format("2006-01-02 15:04:05"),
			unixTimestamp)

		_, err := stmt.Exec(
			unixTimestamp, // Use Unix timestamp instead of time.Time
			rate.Symbol,
			rate.Open,
			rate.High,
			rate.Low,
			rate.Close,
			rate.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert crypto rate: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("SaveCryptoRates: Successfully saved %d rates\n", len(rates))
	return nil
}

// GetCryptoRatesBySymbol retrieves cryptocurrency rates for a specific symbol
func (p *PostgresDB) GetCryptoRatesBySymbol(symbol string, limit int) ([]CryptoRate, error) {
	rows, err := p.db.Query(`
		SELECT id, timestamp, symbol, open, high, low, close, volume, created_at
		FROM crypto_rates
		WHERE symbol = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query crypto rates: %w", err)
	}
	defer rows.Close()

	var rates []CryptoRate
	for rows.Next() {
		var rate CryptoRate
		var timestampUnix int64 // Use int64 to store Unix timestamp

		if err := rows.Scan(
			&rate.ID,
			&timestampUnix, // Scan into Unix timestamp
			&rate.Symbol,
			&rate.Open,
			&rate.High,
			&rate.Low,
			&rate.Close,
			&rate.Volume,
			&rate.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan crypto rate: %w", err)
		}

		// Convert Unix timestamp back to time.Time
		rate.Timestamp = time.Unix(timestampUnix, 0)

		rates = append(rates, rate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over crypto rates: %w", err)
	}

	return rates, nil
}

// GetCryptoRatesByDateRange retrieves cryptocurrency rates for a specific symbol within a date range
func (p *PostgresDB) GetCryptoRatesByDateRange(symbol string, startTime, endTime time.Time) ([]CryptoRate, error) {
	// Convert time.Time to Unix timestamp in seconds
	startUnix := startTime.Unix()
	endUnix := endTime.Unix()

	rows, err := p.db.Query(`
		SELECT id, timestamp, symbol, open, high, low, close, volume, created_at
		FROM crypto_rates
		WHERE symbol = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp DESC
	`, symbol, startUnix, endUnix)
	if err != nil {
		return nil, fmt.Errorf("failed to query crypto rates: %w", err)
	}
	defer rows.Close()

	var rates []CryptoRate
	for rows.Next() {
		var rate CryptoRate
		var timestampUnix int64 // Use int64 to store Unix timestamp

		if err := rows.Scan(
			&rate.ID,
			&timestampUnix, // Scan into Unix timestamp
			&rate.Symbol,
			&rate.Open,
			&rate.High,
			&rate.Low,
			&rate.Close,
			&rate.Volume,
			&rate.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan crypto rate: %w", err)
		}

		// Convert Unix timestamp back to time.Time
		rate.Timestamp = time.Unix(timestampUnix, 0)

		rates = append(rates, rate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over crypto rates: %w", err)
	}

	return rates, nil
}

// GetAvailableCryptoSymbols retrieves a list of available cryptocurrency symbols
func (p *PostgresDB) GetAvailableCryptoSymbols() ([]string, error) {
	rows, err := p.db.Query(`
		SELECT DISTINCT symbol
		FROM crypto_rates
		ORDER BY symbol
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query available symbols: %w", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %w", err)
		}
		symbols = append(symbols, symbol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over symbols: %w", err)
	}

	return symbols, nil
}

// UpdateSchema updates the database schema
func (p *PostgresDB) UpdateSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS telegram_subscriptions (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		currency VARCHAR(10) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(user_id, currency)
	);
	
	CREATE TABLE IF NOT EXISTS crypto_rates (
		id SERIAL PRIMARY KEY,
		timestamp BIGINT NOT NULL,
		symbol VARCHAR(20) NOT NULL,
		open DECIMAL(24, 8) NOT NULL,
		high DECIMAL(24, 8) NOT NULL,
		low DECIMAL(24, 8) NOT NULL,
		close DECIMAL(24, 8) NOT NULL,
		volume DECIMAL(24, 8) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(timestamp, symbol)
	);
	
	CREATE INDEX IF NOT EXISTS idx_crypto_rates_timestamp ON crypto_rates(timestamp);
	CREATE INDEX IF NOT EXISTS idx_crypto_rates_symbol ON crypto_rates(symbol);
	`

	_, err := p.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to update schema: %w", err)
	}

	return nil
}

// TelegramSubscription represents a telegram subscription record in the database
type TelegramSubscription struct {
	ID        int
	UserID    int
	Currency  string
	CreatedAt time.Time
}

// SaveTelegramSubscription saves a telegram subscription to the database
func (p *PostgresDB) SaveTelegramSubscription(userID int, currency string) error {
	_, err := p.db.Exec(`
		INSERT INTO telegram_subscriptions (user_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (user_id, currency) DO NOTHING
	`, userID, currency)
	if err != nil {
		return fmt.Errorf("failed to save telegram subscription: %w", err)
	}
	return nil
}

// DeleteTelegramSubscription deletes a telegram subscription from the database
func (p *PostgresDB) DeleteTelegramSubscription(userID int, currency string) error {
	result, err := p.db.Exec(`
		DELETE FROM telegram_subscriptions
		WHERE user_id = $1 AND currency = $2
	`, userID, currency)
	if err != nil {
		return fmt.Errorf("failed to delete telegram subscription: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("subscription not found")
	}

	return nil
}

// GetTelegramSubscriptions retrieves telegram subscriptions for a specific user
func (p *PostgresDB) GetTelegramSubscriptions(userID int) ([]string, error) {
	rows, err := p.db.Query(`
		SELECT currency
		FROM telegram_subscriptions
		WHERE user_id = $1
		ORDER BY currency
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query telegram subscriptions: %w", err)
	}
	defer rows.Close()

	var currencies []string
	for rows.Next() {
		var currency string
		if err := rows.Scan(&currency); err != nil {
			return nil, fmt.Errorf("failed to scan currency: %w", err)
		}
		currencies = append(currencies, currency)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over telegram subscriptions: %w", err)
	}

	return currencies, nil
}

// GetAllTelegramSubscriptions retrieves all telegram subscriptions
func (p *PostgresDB) GetAllTelegramSubscriptions() (map[int][]string, error) {
	rows, err := p.db.Query(`
		SELECT user_id, currency
		FROM telegram_subscriptions
		ORDER BY user_id, currency
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all telegram subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := make(map[int][]string)
	for rows.Next() {
		var userID int
		var currency string
		if err := rows.Scan(&userID, &currency); err != nil {
			return nil, fmt.Errorf("failed to scan telegram subscription: %w", err)
		}
		subscriptions[userID] = append(subscriptions[userID], currency)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over telegram subscriptions: %w", err)
	}

	return subscriptions, nil
}
