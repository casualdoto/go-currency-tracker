package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/casualdoto/go-currency-tracker/microservices/data-collector/internal/producer"
)

var trackedSymbols = []string{
	"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
	"ADAUSDT", "AVAXUSDT", "DOTUSDT", "DOGEUSDT", "LINKUSDT",
}

// CryptoCollector polls Binance 24hr ticker for tracked symbols.
type CryptoCollector struct {
	prod   *producer.Producer
	client *binance.Client
}

func NewCrypto(prod *producer.Producer) *CryptoCollector {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		},
	}
	bc := binance.NewClient("", "")
	bc.HTTPClient = httpClient

	return &CryptoCollector{prod: prod, client: bc}
}

type rawCryptoRate struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	CollectedAt time.Time `json:"collected_at"`
}

type rawCryptoEvent struct {
	Source string          `json:"source"`
	Rates  []rawCryptoRate `json:"rates"`
}

func (c *CryptoCollector) Collect() error {
	now := time.Now()
	rates := make([]rawCryptoRate, 0, len(trackedSymbols))

	for _, symbol := range trackedSymbols {
		ticker, err := c.client.NewListPriceChangeStatsService().Symbol(symbol).Do(context.Background())
		if err != nil {
			log.Printf("CryptoCollector: failed to get ticker for %s: %v", symbol, err)
			continue
		}
		if len(ticker) == 0 {
			continue
		}
		t := ticker[0]
		open, _ := strconv.ParseFloat(t.OpenPrice, 64)
		high, _ := strconv.ParseFloat(t.HighPrice, 64)
		low, _ := strconv.ParseFloat(t.LowPrice, 64)
		closeP, _ := strconv.ParseFloat(t.LastPrice, 64)
		vol, _ := strconv.ParseFloat(t.Volume, 64)

		rates = append(rates, rawCryptoRate{
			Symbol:      symbol,
			Timestamp:   now,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       closeP,
			Volume:      vol,
			CollectedAt: now,
		})
	}

	if len(rates) == 0 {
		return fmt.Errorf("no crypto rates collected")
	}

	event := rawCryptoEvent{Source: "binance", Rates: rates}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.prod.Publish(ctx, topicRawRates, event); err != nil {
		return fmt.Errorf("crypto publish: %w", err)
	}

	log.Printf("CryptoCollector: published %d rates", len(rates))
	return nil
}
