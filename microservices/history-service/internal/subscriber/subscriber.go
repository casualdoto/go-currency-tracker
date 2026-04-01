package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
	"github.com/segmentio/kafka-go"
)

const (
	topicNormalizedRates = "normalized-rates"
	groupID              = "history-service"
)

type Subscriber struct {
	reader *kafka.Reader
	db     *storage.PostgresDB
}

func New(brokers string, db *storage.PostgresDB) *Subscriber {
	brokerList := strings.Split(brokers, ",")
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokerList,
		Topic:    topicNormalizedRates,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Subscriber{reader: r, db: db}
}

func (s *Subscriber) Run() error {
	ctx := context.Background()
	for {
		msg, err := s.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		if err := s.process(msg.Value); err != nil {
			log.Printf("subscriber: process error: %v", err)
		}
	}
}

type baseEvent struct {
	Source string          `json:"source"`
	Rates  json.RawMessage `json:"rates"`
}

type normalizedCBRRate struct {
	Date         time.Time `json:"date"`
	CurrencyCode string    `json:"currency_code"`
	CurrencyName string    `json:"currency_name"`
	Nominal      int       `json:"nominal"`
	ValueRUB     float64   `json:"value_rub"`
	PreviousRUB  float64   `json:"previous_rub"`
}

type normalizedCryptoRate struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	PriceRUB  float64   `json:"price_rub"`
}

func (s *Subscriber) process(data []byte) error {
	var evt baseEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	switch evt.Source {
	case "cbr":
		var rates []normalizedCBRRate
		if err := json.Unmarshal(evt.Rates, &rates); err != nil {
			return err
		}
		dbRates := make([]storage.CurrencyRate, 0, len(rates))
		for _, r := range rates {
			dbRates = append(dbRates, storage.CurrencyRate{
				Date:         r.Date,
				CurrencyCode: r.CurrencyCode,
				CurrencyName: r.CurrencyName,
				Nominal:      r.Nominal,
				Value:        r.ValueRUB,
				Previous:     r.PreviousRUB,
			})
		}
		if err := s.db.SaveCurrencyRates(dbRates); err != nil {
			return err
		}
		log.Printf("subscriber: saved %d CBR rates", len(dbRates))

	case "binance":
		var rates []normalizedCryptoRate
		if err := json.Unmarshal(evt.Rates, &rates); err != nil {
			return err
		}
		dbRates := make([]storage.CryptoRate, 0, len(rates))
		for _, r := range rates {
			dbRates = append(dbRates, storage.CryptoRate{
				Timestamp: r.Timestamp,
				Symbol:    r.Symbol,
				Open:      r.Open,
				High:      r.High,
				Low:       r.Low,
				Close:     r.Close,
				Volume:    r.Volume,
				PriceRUB:  r.PriceRUB,
			})
		}
		if err := s.db.SaveCryptoRates(dbRates); err != nil {
			return err
		}
		log.Printf("subscriber: saved %d crypto rates", len(dbRates))
	}
	return nil
}
