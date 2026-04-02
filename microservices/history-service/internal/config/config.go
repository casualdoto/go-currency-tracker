package config

import "os"

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

		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		ServerPort:    getEnv("SERVER_PORT", "8084"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
