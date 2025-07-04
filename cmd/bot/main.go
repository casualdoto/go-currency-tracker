package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
	"github.com/casualdoto/go-currency-tracker/internal/scheduler"
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

	// Create a new Telegram bot
	bot, err := alert.NewTelegramBot(token)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Start the bot
	bot.Start()
	log.Println("Telegram bot started")

	// Create a scheduler for daily updates at 10:00 AM
	sched := scheduler.NewTelegramScheduler(bot)
	sched.StartDailyUpdates(10)
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
