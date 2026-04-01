package config

import "os"

type Config struct {
	RedisAddr        string
	KafkaBrokers     string
	TelegramBotToken string
	ServerPort       string
}

func Load() *Config {
	return &Config{
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:     getEnv("KAFKA_BROKERS", "localhost:9092"),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		ServerPort:       getEnv("SERVER_PORT", "8085"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
