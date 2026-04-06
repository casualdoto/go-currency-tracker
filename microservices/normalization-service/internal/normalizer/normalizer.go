package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	topicRawRates        = "raw-rates"
	topicNormalizedRates = "normalized-rates"
	groupID              = "normalization-service"
)

// Normalizer reads from raw-rates, normalizes, and publishes to normalized-rates.
type Normalizer struct {
	reader      *kafka.Reader
	writer      *kafka.Writer
	cbrURL      string
	httpClient  *http.Client
	lastUSDRUB  float64 // last successfully fetched USD/RUB rate; used as fallback
}

func New(brokers, cbrURL string) *Normalizer {
	brokerList := strings.Split(brokers, ",")
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokerList,
		Topic:    topicRawRates,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokerList...),
		Topic:        topicNormalizedRates,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}
	return &Normalizer{
		reader:     r,
		writer:     w,
		cbrURL:     cbrURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *Normalizer) Run() error {
	ctx := context.Background()
	for {
		msg, err := n.reader.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}
		if err := n.process(ctx, msg.Value); err != nil {
			log.Printf("normalizer: process error: %v", err)
		}
	}
}

type rawEvent struct {
	Source string          `json:"source"`
	Rates  json.RawMessage `json:"rates"`
}

type rawCBRRate struct {
	Date     string  `json:"date"`
	CharCode string  `json:"char_code"`
	NumCode  string  `json:"num_code"`
	Nominal  int     `json:"nominal"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Previous float64 `json:"previous"`
}

type rawCryptoRate struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

type normalizedCBRRate struct {
	Date         time.Time `json:"date"`
	CurrencyCode string    `json:"currency_code"`
	CurrencyName string    `json:"currency_name"`
	Nominal      int       `json:"nominal"`
	ValueRUB     float64   `json:"value_rub"`
	PreviousRUB  float64   `json:"previous_rub"`
}

type normalizedCBREvent struct {
	Source string              `json:"source"`
	Rates  []normalizedCBRRate `json:"rates"`
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

type normalizedCryptoEvent struct {
	Source string                 `json:"source"`
	Rates  []normalizedCryptoRate `json:"rates"`
}

func (n *Normalizer) process(ctx context.Context, data []byte) error {
	var evt rawEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	switch evt.Source {
	case "cbr":
		return n.normalizeCBR(ctx, evt.Rates)
	case "binance":
		return n.normalizeCrypto(ctx, evt.Rates)
	default:
		log.Printf("normalizer: unknown source %q", evt.Source)
	}
	return nil
}

func (n *Normalizer) normalizeCBR(ctx context.Context, raw json.RawMessage) error {
	normalized, err := n.buildNormalizedCBR(raw)
	if err != nil {
		return err
	}
	return n.publish(ctx, normalizedCBREvent{Source: "cbr", Rates: normalized})
}

// buildNormalizedCBR parses raw CBR rates and returns normalized structs.
// Extracted for unit-testability.
func (n *Normalizer) buildNormalizedCBR(raw json.RawMessage) ([]normalizedCBRRate, error) {
	var rates []rawCBRRate
	if err := json.Unmarshal(raw, &rates); err != nil {
		return nil, err
	}

	normalized := make([]normalizedCBRRate, 0, len(rates))
	for _, r := range rates {
		date, err := time.Parse("2006/01/02 15:04:05", r.Date)
		if err != nil {
			date, err = time.Parse("2006-01-02T15:04:05-07:00", r.Date)
			if err != nil {
				date = time.Now().Truncate(24 * time.Hour)
			}
		}
		normalized = append(normalized, normalizedCBRRate{
			Date:         date,
			CurrencyCode: r.CharCode,
			CurrencyName: r.Name,
			Nominal:      r.Nominal,
			ValueRUB:     r.Value,
			PreviousRUB:  r.Previous,
		})
	}
	return normalized, nil
}

func (n *Normalizer) normalizeCrypto(ctx context.Context, raw json.RawMessage) error {
	normalized, err := n.buildNormalizedCrypto(raw)
	if err != nil {
		return err
	}
	return n.publish(ctx, normalizedCryptoEvent{Source: "binance", Rates: normalized})
}

// buildNormalizedCrypto fetches USD/RUB, calculates PriceRUB and returns
// normalized structs. Extracted for unit-testability.
func (n *Normalizer) buildNormalizedCrypto(raw json.RawMessage) ([]normalizedCryptoRate, error) {
	var rates []rawCryptoRate
	if err := json.Unmarshal(raw, &rates); err != nil {
		return nil, err
	}

	usdRUB, err := n.getUSDRUBRate()
	if err != nil {
		if n.lastUSDRUB != 0 {
			log.Printf("normalizer: failed to get USD/RUB rate: %v, using last known rate %.4f", err, n.lastUSDRUB)
			usdRUB = n.lastUSDRUB
		} else {
			log.Printf("normalizer: failed to get USD/RUB rate: %v, no cached rate available, using 1.0", err)
			usdRUB = 1.0
		}
	} else {
		n.lastUSDRUB = usdRUB
	}

	normalized := make([]normalizedCryptoRate, 0, len(rates))
	for _, r := range rates {
		normalized = append(normalized, normalizedCryptoRate{
			Symbol:    r.Symbol,
			Timestamp: r.Timestamp,
			Open:      r.Open,
			High:      r.High,
			Low:       r.Low,
			Close:     r.Close,
			Volume:    r.Volume,
			PriceRUB:  r.Close * usdRUB,
		})
	}
	return normalized, nil
}

type cbrResp struct {
	Valute map[string]struct {
		Value float64 `json:"Value"`
	} `json:"Valute"`
}

func (n *Normalizer) getUSDRUBRate() (float64, error) {
	resp, err := n.httpClient.Get(n.cbrURL + "/daily_json.js")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var data cbrResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	usd, ok := data.Valute["USD"]
	if !ok {
		return 0, fmt.Errorf("USD not found in CBR response")
	}
	return usd.Value, nil
}

func (n *Normalizer) publish(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return n.writer.WriteMessages(ctx, kafka.Message{Value: data})
}
