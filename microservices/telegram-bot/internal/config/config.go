package config

import "os"

type Config struct {
	TelegramBotToken   string
	APIGatewayURL      string
	NotificationSvcURL string
	RedisAddr          string
}

func Load() *Config {
	return &Config{
		TelegramBotToken:   getEnv("TELEGRAM_BOT_TOKEN", ""),
		APIGatewayURL:      getEnv("API_GATEWAY_URL", "http://localhost:8080"),
		NotificationSvcURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8085"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
