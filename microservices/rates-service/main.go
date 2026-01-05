package main

import (
	"fmt"
	"net/http"

	"github.com/currency-tracker/go-currency-tracker/microservices/shared/config"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Configure logger
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	logrus.Info("Starting Rates Service")

	// Simple health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "rates-service"}`))
	})

	// TODO: Initialize database connection
	// TODO: Initialize Redis connection
	// TODO: Initialize Kafka consumer/producer
	// TODO: Add actual API endpoints

	// Start HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	logrus.WithField("addr", addr).Info("Rates Service listening")

	if err := http.ListenAndServe(addr, nil); err != nil {
		logrus.WithError(err).Fatal("Failed to start HTTP server")
	}
}
