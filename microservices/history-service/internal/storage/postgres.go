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
	`)
	return err
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
