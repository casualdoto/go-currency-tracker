package config

import "os"

type Config struct {
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	KafkaBrokers string
	ServerPort   string
}

func Load() *Config {
	return &Config{
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "currency_user"),
		DBPassword:   getEnv("DB_PASSWORD", "currency_password"),
		DBName:       getEnv("DB_NAME", "currency_db"),
		DBSSLMode:    getEnv("DB_SSLMODE", "disable"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		ServerPort:   getEnv("SERVER_PORT", "8084"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
