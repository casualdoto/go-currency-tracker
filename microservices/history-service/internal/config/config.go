package config

import (
	"os"
	"strings"
)

type Config struct {
	// PostgreSQL (CBR rates)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// ClickHouse (crypto rates)
	CHHost     string
	CHPort     string
	CHDatabase string
	CHUser     string
	CHPassword string

	KafkaBrokers string
	ServerPort   string

	// CBRBaseURL is used to pull missing archive daily_json into PostgreSQL (same host as data-collector).
	CBRBaseURL string
	// BinanceAPIBase is the REST root for klines backfill (empty = https://api.binance.com).
	BinanceAPIBase string
}

func Load() *Config {
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "currency_user"),
		DBPassword: getEnv("DB_PASSWORD", "currency_password"),
		DBName:     getEnv("DB_NAME", "currency_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		CHHost:     getEnv("CH_HOST", "localhost"),
		CHPort:     getEnv("CH_PORT", "9000"),
		CHDatabase: getEnv("CH_DATABASE", "default"),
		CHUser:     getEnv("CH_USER", "default"),
		CHPassword: getEnv("CH_PASSWORD", ""),

		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),
		ServerPort:     getEnv("SERVER_PORT", "8084"),
		CBRBaseURL:     getEnvAllowEmpty("CBR_BASE_URL", "https://www.cbr-xml-daily.ru"),
		BinanceAPIBase: strings.TrimSpace(os.Getenv("BINANCE_API_BASE")),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvAllowEmpty returns def if the variable is unset; if set to empty string, returns "" (disables CBR backfill).
func getEnvAllowEmpty(key, def string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	return strings.TrimSpace(v)
}
