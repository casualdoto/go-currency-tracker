package config

import "os"

type Config struct {
	HistoryServiceURL      string
	NotificationServiceURL string
	ServerPort             string
}

func Load() *Config {
	return &Config{
		HistoryServiceURL:      getEnv("HISTORY_SERVICE_URL", "http://localhost:8084"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8085"),
		ServerPort:             getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
