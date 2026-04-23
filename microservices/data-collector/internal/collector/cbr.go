package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/producer"
)

const topicRawRates = "raw-rates"

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
	Date   string                 `json:"Date"`
	Valute map[string]cbrValute   `json:"Valute"`
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

type rawCBRRate struct {
	Date        string    `json:"date"`
	CharCode    string    `json:"char_code"`
	NumCode     string    `json:"num_code"`
	Nominal     int       `json:"nominal"`
	Name        string    `json:"name"`
	Value       float64   `json:"value"`
	Previous    float64   `json:"previous"`
	CollectedAt time.Time `json:"collected_at"`
}

type rawCBREvent struct {
	Source string       `json:"source"`
	Rates  []rawCBRRate `json:"rates"`
}

// parseCBRResponse converts a decoded CBR API response into a slice of rawCBRRate.
// Pure function — no I/O, directly testable.
func parseCBRResponse(data cbrResponse, collectedAt time.Time) []rawCBRRate {
	rates := make([]rawCBRRate, 0, len(data.Valute))
	for _, v := range data.Valute {
		rates = append(rates, rawCBRRate{
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

	event := rawCBREvent{Source: "cbr", Rates: rates}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.prod.Publish(ctx, topicRawRates, event); err != nil {
		return fmt.Errorf("cbr publish: %w", err)
	}

	log.Printf("CBRCollector: published %d rates for date %s", len(rates), data.Date)
	return nil
}
