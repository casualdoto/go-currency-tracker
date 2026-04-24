package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/casualdoto/go-currency-tracker/microservices/history-service/internal/storage"
	"github.com/casualdoto/go-currency-tracker/microservices/shared/events"
	"github.com/segmentio/kafka-go"
)

const groupID = "history-service"

type Subscriber struct {
	reader *kafka.Reader
	pg     *storage.PostgresDB
	ch     *storage.ClickHouseDB
}

func New(brokers string, pg *storage.PostgresDB, ch *storage.ClickHouseDB) *Subscriber {
	brokerList := strings.Split(brokers, ",")
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokerList,
		Topic:    events.TopicNormalizedRates,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Subscriber{reader: r, pg: pg, ch: ch}
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

func (s *Subscriber) process(data []byte) error {
	var evt baseEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	switch evt.Source {
	case string(events.SourceCBR):
		var rates []events.NormalizedCBRRate
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
		if err := s.pg.SaveCurrencyRates(dbRates); err != nil {
			return err
		}
		log.Printf("subscriber: saved %d CBR rates to PostgreSQL", len(dbRates))

	case string(events.SourceBinance):
		var rates []events.NormalizedCryptoRate
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
		if err := s.ch.SaveCryptoRates(dbRates); err != nil {
			return err
		}
		log.Printf("subscriber: saved %d crypto rates to ClickHouse", len(dbRates))
	}
	return nil
}
