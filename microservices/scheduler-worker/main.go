package main

import (
	"context"

	"github.com/currency-tracker/go-currency-tracker/microservices/shared/config"
	"github.com/currency-tracker/go-currency-tracker/microservices/shared/events"
	"github.com/robfig/cron/v3"
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

	logrus.Info("Starting Scheduler Worker")

	// Initialize Kafka writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    string(events.RatesRefreshRequested),
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Create cron scheduler
	c := cron.New()

	// Schedule rate refresh every 5 minutes
	_, err = c.AddFunc("*/5 * * * *", func() {
		publishRefreshRequest(writer)
	})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to schedule refresh job")
	}

	logrus.Info("Scheduler started - refreshing rates every 5 minutes")

	// Start cron
	c.Start()

	// Keep running
	select {}
}

// publishRefreshRequest publishes a rates refresh request event to Kafka
func publishRefreshRequest(writer *kafka.Writer) {
	event := events.NewEvent(events.RatesRefreshRequested, events.RatesRefreshRequestedPayload{})

	eventJSON, err := event.ToJSON()
	if err != nil {
		logrus.WithError(err).Error("Failed to serialize refresh event")
		return
	}

	err = writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(event.ID),
			Value: eventJSON,
		},
	)

	if err != nil {
		logrus.WithError(err).Error("Failed to publish refresh request")
	} else {
		logrus.WithField("event_id", event.ID).Info("Published rates refresh request")
	}
}
