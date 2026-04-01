package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/collector"
	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/producer"
)

func main() {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	cbrURL := getEnv("CBR_BASE_URL", "https://www.cbr-xml-daily.ru")
	cbrInterval := getDurationEnv("COLLECT_INTERVAL_CBR", 86400) // daily
	cryptoInterval := getDurationEnv("COLLECT_INTERVAL_CRYPTO", 60) // every minute

	p := producer.New(brokers)
	defer p.Close()

	cbrCollector := collector.NewCBR(cbrURL, p)
	cryptoCollector := collector.NewCrypto(p)

	// Run CBR collector
	go func() {
		log.Println("Data Collector: starting CBR polling every", cbrInterval)
		// Run immediately, then on schedule
		if err := cbrCollector.Collect(); err != nil {
			log.Printf("CBR collect error: %v", err)
		}
		t := time.NewTicker(cbrInterval)
		defer t.Stop()
		for range t.C {
			if err := cbrCollector.Collect(); err != nil {
				log.Printf("CBR collect error: %v", err)
			}
		}
	}()

	// Run Crypto collector
	go func() {
		log.Println("Data Collector: starting Crypto polling every", cryptoInterval)
		if err := cryptoCollector.Collect(); err != nil {
			log.Printf("Crypto collect error: %v", err)
		}
		t := time.NewTicker(cryptoInterval)
		defer t.Stop()
		for range t.C {
			if err := cryptoCollector.Collect(); err != nil {
				log.Printf("Crypto collect error: %v", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Data Collector: shutting down")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDurationEnv(key string, defaultSeconds int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(defaultSeconds) * time.Second
}
