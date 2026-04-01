package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// Config holds all configuration variables
type Config struct {
	CBRBaseURL       string
	TelegramBotToken string
	TelegramChatID   string
	DBHost           string
	DBPort           string
	DBName           string
	DBUser           string
	DBPassword       string
	DBSSLMode        string
	ServerPort       string
	APIPort          string
}

var (
	config *Config
	once   sync.Once
)

// Load loads configuration from .env file in project root
func Load() *Config {
	once.Do(func() {
		config = &Config{}
		loadEnvironment()
		loadFromEnv()
	})
	return config
}

// Get returns the loaded configuration
func Get() *Config {
	if config == nil {
		return Load()
	}
	return config
}

// loadEnvironment loads .env file from project root
func loadEnvironment() {
	// Find project root by looking for go.mod file
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		log.Println("Warning: Could not find project root, using current directory")
		projectRoot = "."
	}

	envPath := filepath.Join(projectRoot, ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		log.Printf("Warning: Could not load .env file from %s: %v", envPath, err)
	} else {
		log.Printf("Successfully loaded .env file from %s", envPath)
	}
}

// findProjectRoot finds the project root directory by looking for go.mod file
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Look for go.mod file going up the directory tree
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	return ""
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv() {
	config.CBRBaseURL = getEnvWithDefault("CBR_BASE_URL", "https://www.cbr-xml-daily.ru")
	config.TelegramBotToken = getEnvWithDefault("TELEGRAM_BOT_TOKEN", "")
	config.TelegramChatID = getEnvWithDefault("TELEGRAM_CHAT_ID", "")
	config.DBHost = getEnvWithDefault("DB_HOST", "localhost")
	config.DBPort = getEnvWithDefault("DB_PORT", "5432")
	config.DBName = getEnvWithDefault("DB_NAME", "currency_tracker")
	config.DBUser = getEnvWithDefault("DB_USER", "postgres")
	config.DBPassword = getEnvWithDefault("DB_PASSWORD", "postgres")
	config.DBSSLMode = getEnvWithDefault("DB_SSLMODE", "disable")
	config.ServerPort = getEnvWithDefault("SERVER_PORT", "8080")
	config.APIPort = getEnvWithDefault("API_PORT", "8081")

	// Clean up URLs by removing quotes if they exist
	config.CBRBaseURL = strings.Trim(config.CBRBaseURL, `"`)

	log.Printf("Configuration loaded - CBR_BASE_URL: %s", config.CBRBaseURL)
}

// getEnvWithDefault gets environment variable with default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetCBRBaseURL returns CBR base URL
func GetCBRBaseURL() string {
	return Get().CBRBaseURL
}

// GetTelegramBotToken returns Telegram bot token
func GetTelegramBotToken() string {
	return Get().TelegramBotToken
}

// SetCBRBaseURLForTesting sets CBR base URL for testing purposes
func SetCBRBaseURLForTesting(url string) {
	if config == nil {
		config = &Config{}
	}
	config.CBRBaseURL = url
}

// GetDBConnectionString returns database connection string
func GetDBConnectionString() string {
	cfg := Get()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
}
