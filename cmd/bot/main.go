package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
	"github.com/casualdoto/go-currency-tracker/internal/currency/binance"
	"github.com/casualdoto/go-currency-tracker/internal/scheduler"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found. Using environment variables.")
	}

	// Get Telegram bot token from environment variables
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set in environment variables")
	}

	// Setup database connection
	dbConfig := storage.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvAsInt("DB_PORT", 5432),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "currency_tracker"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	db, err := storage.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize database schema
	if err := db.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Create a new Telegram bot
	bot, err := alert.NewTelegramBot(token, db)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Start the bot
	bot.Start()
	log.Println("Telegram bot started")

	// Test crypto rates functionality
	log.Println("Testing crypto rates...")
	testClient := binance.NewClient()

	// Test BTC
	btcRate, err := testClient.GetCurrentCryptoToRubRate("BTC")
	if err != nil {
		log.Printf("Error getting BTC rate: %v", err)
	} else {
		log.Printf("BTC rate test: %.2f RUB", btcRate.Close)
	}

	// Test DOGE
	dogeRate, err := testClient.GetCurrentCryptoToRubRate("DOGE")
	if err != nil {
		log.Printf("Error getting DOGE rate: %v", err)
	} else {
		log.Printf("DOGE rate test: %.2f RUB", dogeRate.Close)
	}

	// Create a scheduler for daily updates at 2:00 UTC
	sched := scheduler.NewTelegramScheduler(bot)
	sched.StartDailyUpdates(2)
	// for test
	//sched.RunNow()
	log.Println("Daily updates scheduler started")

	// Start crypto updates every 15 minutes
	sched.StartCryptoUpdates()
	log.Println("Crypto updates scheduler started (15 minutes interval)")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Stop the bot and scheduler
	log.Println("Shutting down...")
	sched.Stop()
	bot.Stop()
	log.Println("Bot stopped")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Could not parse %s as integer, using default value %d", key, defaultValue)
		return defaultValue
	}
	return value
}
