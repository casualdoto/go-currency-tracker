package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/api"
	"github.com/casualdoto/go-currency-tracker/internal/scheduler"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
)

func main() {
	// Define working directory for correct access to static files
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	log.Printf("Working directory: %s", workDir)

	// Check for api directory
	apiDir := filepath.Join(workDir, "api")
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		log.Printf("Warning: API docs directory not found at %s", apiDir)
	} else {
		log.Printf("API docs directory found at %s", apiDir)

		// Check for documentation file
		apiFile := filepath.Join(apiDir, "openapi.json")
		if _, err := os.Stat(apiFile); os.IsNotExist(err) {
			log.Printf("Warning: API docs file not found at %s", apiFile)
		} else {
			log.Printf("API docs file found at %s", apiFile)
		}
	}

	// Initialize database connection
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize scheduler for daily currency rate updates at 23:59 UTC
	currencyScheduler := scheduler.NewCurrencyRateScheduler(db, 23, 59)
	currencyScheduler.Start()
	defer currencyScheduler.Stop()

	// Run the initial currency rates update
	log.Println("Running initial currency rates update...")
	if err := currencyScheduler.RunImmediately(); err != nil {
		log.Printf("Warning: Failed to run initial currency rates update: %v", err)
	} else {
		log.Println("Initial currency rates update completed successfully")
	}

	// Setup routes with database access
	router := api.SetupRoutesWithDB(db)

	// Start HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Println("Go Currency Monitor API started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}

// initDatabase initializes the database connection and schema
func initDatabase() (*storage.PostgresDB, error) {
	// Try to get database configuration from environment variables
	// or use default values
	dbConfig := storage.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432, // Default PostgreSQL port
		User:     getEnv("DB_USER", "currency_user"),
		Password: getEnv("DB_PASSWORD", "currency_password"),
		DBName:   getEnv("DB_NAME", "currency_db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	// Create a new database connection with retry mechanism
	var db *storage.PostgresDB
	var err error

	maxRetries := 5
	retryDelay := time.Second * 3

	for i := 0; i < maxRetries; i++ {
		db, err = storage.NewPostgresDB(dbConfig)
		if err == nil {
			break
		}

		log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			// Increase delay for next attempt
			retryDelay *= 2
		}
	}

	if err != nil {
		return nil, err
	}

	// Initialize database schema
	if err := db.InitSchema(); err != nil {
		return nil, err
	}

	log.Println("Database connection established and schema initialized")
	return db, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
