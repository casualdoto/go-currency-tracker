package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/shared/events"
	"github.com/segmentio/kafka-go"
)

const (
	groupID = "normalization-service"
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
		Topic:    events.TopicRawRates,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokerList...),
		Topic:        events.TopicNormalizedRates,
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

func (n *Normalizer) process(ctx context.Context, data []byte) error {
	var evt rawEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	switch evt.Source {
	case string(events.SourceCBR):
		return n.normalizeCBR(ctx, evt.Rates)
	case string(events.SourceBinance):
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
	return n.publish(ctx, events.NormalizedCBRRatesEvent{Source: events.SourceCBR, Rates: normalized})
}

// buildNormalizedCBR parses raw CBR rates and returns normalized structs.
// Extracted for unit-testability.
func (n *Normalizer) buildNormalizedCBR(raw json.RawMessage) ([]events.NormalizedCBRRate, error) {
	var rates []events.RawCBRRate
	if err := json.Unmarshal(raw, &rates); err != nil {
		return nil, err
	}

	normalized := make([]events.NormalizedCBRRate, 0, len(rates))
	for _, r := range rates {
		date, err := time.Parse("2006/01/02 15:04:05", r.Date)
		if err != nil {
			date, err = time.Parse("2006-01-02T15:04:05-07:00", r.Date)
			if err != nil {
				date = time.Now().Truncate(24 * time.Hour)
			}
		}
		normalized = append(normalized, events.NormalizedCBRRate{
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
	return n.publish(ctx, events.NormalizedCryptoRatesEvent{Source: events.SourceBinance, Rates: normalized})
}

// buildNormalizedCrypto fetches USD/RUB, calculates PriceRUB and returns
// normalized structs. Extracted for unit-testability.
func (n *Normalizer) buildNormalizedCrypto(raw json.RawMessage) ([]events.NormalizedCryptoRate, error) {
	var rates []events.RawCryptoRate
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

	normalized := make([]events.NormalizedCryptoRate, 0, len(rates))
	for _, r := range rates {
		normalized = append(normalized, events.NormalizedCryptoRate{
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
