package main

import (
	"context"

	"github.com/currency-tracker/go-currency-tracker/microservices/shared/config"
	"github.com/segmentio/kafka-go"
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

	logrus.Info("Starting CBR Worker")

	// Create Kafka reader for refresh requests
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   string("rates.refresh.requested"),
		GroupID: "cbr-worker-group",
	})
	defer reader.Close()

	logrus.Info("CBR Worker started - listening for refresh requests")

	// Listen for refresh events
	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			logrus.WithError(err).Error("Failed to read message")
			continue
		}

		logrus.WithField("offset", msg.Offset).Info("Received refresh request")

		// TODO: Fetch rates from CBR API
		// TODO: Normalize data
		// TODO: Publish rates.source.updated event
	}
}
