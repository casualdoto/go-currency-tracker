package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/normalization-service/internal/normalizer"
)

func main() {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	cbrURL := getEnv("CBR_BASE_URL", "https://www.cbr-xml-daily.ru")

	svc := normalizer.New(brokers, cbrURL)

	go func() {
		log.Println("Normalization Service: starting")
		if err := svc.Run(); err != nil {
			log.Fatalf("normalizer error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Normalization Service: shutting down")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
