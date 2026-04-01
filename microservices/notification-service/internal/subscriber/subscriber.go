package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/store"
	"github.com/segmentio/kafka-go"
)

const (
	topicNormalizedRates = "normalized-rates"
	groupID              = "notification-service"
)

type Subscriber struct {
	reader     *kafka.Reader
	store      *store.RedisStore
	botToken   string
	httpClient *http.Client
}

func New(brokers string, s *store.RedisStore, botToken string) *Subscriber {
	brokerList := strings.Split(brokers, ",")
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokerList,
		Topic:    topicNormalizedRates,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Subscriber{
		reader:     r,
		store:      s,
		botToken:   botToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Subscriber) Run() error {
	ctx := context.Background()
	for {
		msg, err := s.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		if err := s.process(ctx, msg.Value); err != nil {
			log.Printf("notification subscriber: process error: %v", err)
		}
	}
}

type baseEvent struct {
	Source string          `json:"source"`
	Rates  json.RawMessage `json:"rates"`
}

type normalizedCryptoRate struct {
	Symbol   string  `json:"symbol"`
	PriceRUB float64 `json:"price_rub"`
}

func (s *Subscriber) process(ctx context.Context, data []byte) error {
	var evt baseEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	if evt.Source != "binance" {
		return nil // only notify on crypto changes for now
	}

	var rates []normalizedCryptoRate
	if err := json.Unmarshal(evt.Rates, &rates); err != nil {
		return err
	}

	subscribers, err := s.store.GetAllCryptoSubscribers(ctx)
	if err != nil {
		return err
	}

	for _, rate := range rates {
		// Strip USDT suffix for matching (e.g. BTCUSDT -> BTC)
		symbol := strings.TrimSuffix(rate.Symbol, "USDT")
		tids, ok := subscribers[symbol]
		if !ok {
			continue
		}
		msg := fmt.Sprintf("💰 %s update: %.2f RUB", symbol, rate.PriceRUB)
		for _, tid := range tids {
			s.sendTelegram(tid, msg)
		}
	}
	return nil
}

func (s *Subscriber) sendTelegram(chatID int64, text string) {
	if s.botToken == "" {
		return
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	body := fmt.Sprintf(`{"chat_id":%d,"text":%q}`, chatID, text)
	resp, err := s.httpClient.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("sendTelegram: %v", err)
		return
	}
	resp.Body.Close()
}
