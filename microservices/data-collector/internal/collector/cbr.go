package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/producer"
	"github.com/casualdoto/go-currency-tracker/microservices/shared/events"
)

// CBRCollector polls the CBR API and publishes raw CBR rates to Kafka.
type CBRCollector struct {
	baseURL string
	prod    *producer.Producer
	client  *http.Client
}

func NewCBR(baseURL string, prod *producer.Producer) *CBRCollector {
	return &CBRCollector{
		baseURL: baseURL,
		prod:    prod,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type cbrResponse struct {
	Date   string               `json:"Date"`
	Valute map[string]cbrValute `json:"Valute"`
}

type cbrValute struct {
	ID       string  `json:"ID"`
	NumCode  string  `json:"NumCode"`
	CharCode string  `json:"CharCode"`
	Nominal  int     `json:"Nominal"`
	Name     string  `json:"Name"`
	Value    float64 `json:"Value"`
	Previous float64 `json:"Previous"`
}

// parseCBRResponse converts a decoded CBR API response into a slice of RawCBRRate.
// Pure function — no I/O, directly testable.
func parseCBRResponse(data cbrResponse, collectedAt time.Time) []events.RawCBRRate {
	rates := make([]events.RawCBRRate, 0, len(data.Valute))
	for _, v := range data.Valute {
		rates = append(rates, events.RawCBRRate{
			Date:        data.Date,
			CharCode:    v.CharCode,
			NumCode:     v.NumCode,
			Nominal:     v.Nominal,
			Name:        v.Name,
			Value:       v.Value,
			Previous:    v.Previous,
			CollectedAt: collectedAt,
		})
	}
	return rates
}

func (c *CBRCollector) Collect() error {
	url := fmt.Sprintf("%s/daily_json.js", c.baseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("cbr fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cbr status %d", resp.StatusCode)
	}

	var data cbrResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("cbr decode: %w", err)
	}

	now := time.Now()
	rates := parseCBRResponse(data, now)

	event := events.RawCBRRatesEvent{Source: events.SourceCBR, Rates: rates}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.prod.Publish(ctx, events.TopicRawRates, event); err != nil {
		return fmt.Errorf("cbr publish: %w", err)
	}

	log.Printf("CBRCollector: published %d rates for date %s", len(rates), data.Date)
	return nil
}
