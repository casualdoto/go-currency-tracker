package config

import "os"

type Config struct {
	AuthServiceURL         string
	HistoryServiceURL      string
	NotificationServiceURL string
	JWTSecret              string
	ServerPort             string
}

func Load() *Config {
	return &Config{
		AuthServiceURL:         getEnv("AUTH_SERVICE_URL", "http://localhost:8082"),
		HistoryServiceURL:      getEnv("HISTORY_SERVICE_URL", "http://localhost:8084"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8085"),
		JWTSecret:              getEnv("JWT_SECRET", "supersecretjwtkey"),
		ServerPort:             getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
