package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the microservices
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port string
	Host string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers []string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Server config
	config.Server = ServerConfig{
		Port: getEnv("PORT", "8080"),
		Host: getEnv("HOST", "0.0.0.0"),
	}

	// Database config
	dbPort, _ := strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	config.Database = DatabaseConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     dbPort,
		User:     getEnv("POSTGRES_USER", "currency_user"),
		Password: getEnv("POSTGRES_PASSWORD", "currency_pass"),
		DBName:   getEnv("POSTGRES_DB", "currency_tracker"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	// Redis config
	redisPort, _ := strconv.Atoi(getEnv("REDIS_PORT", "6379"))
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	config.Redis = RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     redisPort,
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       redisDB,
	}

	// Kafka config
	config.Kafka = KafkaConfig{
		Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
	}

	return config, nil
}

// GetDatabaseURL returns PostgreSQL connection string
func (c *DatabaseConfig) GetDatabaseURL() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
