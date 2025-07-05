package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
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

	// Create a scheduler for daily updates at 16:00
	sched := scheduler.NewTelegramScheduler(bot)
	sched.StartDailyUpdates(16)
	// for test
	//sched.RunNow()
	log.Println("Daily updates scheduler started")

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
