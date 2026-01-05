package events

import (
	"encoding/json"
	"time"

	"github.com/currency-tracker/go-currency-tracker/microservices/shared/types"
)

// EventType represents different types of events
type EventType string

const (
	// RatesRefreshRequested is sent by scheduler to request rate updates
	RatesRefreshRequested EventType = "rates.refresh.requested"

	// RatesSourceUpdated is sent by workers when new rates are fetched
	RatesSourceUpdated EventType = "rates.source.updated"

	// RatesUpdated is sent by rates service when rates are persisted
	RatesUpdated EventType = "rates.updated"
)

// Event represents a Kafka event
type Event struct {
	ID        string      `json:"id"`
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// RatesRefreshRequestedPayload is empty for refresh requests
type RatesRefreshRequestedPayload struct{}

// RatesSourceUpdatedPayload contains updated rates from external sources
type RatesSourceUpdatedPayload struct {
	Source       string              `json:"source"` // 'cbr' or 'binance'
	CurrencyRates []types.CurrencyRate `json:"currency_rates,omitempty"`
	CryptoRates   []types.CryptoRate   `json:"crypto_rates,omitempty"`
}

// RatesUpdatedPayload contains confirmation of persisted rates
type RatesUpdatedPayload struct {
	Source    string    `json:"source"`
	Count     int       `json:"count"`
	Timestamp time.Time `json:"timestamp"`
}

// NewEvent creates a new event with generated ID and timestamp
func NewEvent(eventType EventType, payload interface{}) *Event {
	return &Event{
		ID:        generateID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// ToJSON serializes event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	err := json.Unmarshal(data, &event)
	return &event, err
}

// generateID generates a simple ID for events (in production, use UUID)
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + "event"
}
