package currency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Структуры для парсинга ответа API
type DailyRates struct {
	Date   string            `json:"Date"`
	Valute map[string]Valute `json:"Valute"`
}

type Valute struct {
	ID       string  `json:"ID"`
	NumCode  string  `json:"NumCode"`
	CharCode string  `json:"CharCode"`
	Nominal  int     `json:"Nominal"`
	Name     string  `json:"Name"`
	Value    float64 `json:"Value"`
	Previous float64 `json:"Previous"`
}

// Получить курсы с сайта ЦБ
func GetCBRRates() (*DailyRates, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := "https://www.cbr-xml-daily.ru/daily_json.js"

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CBR rates: %w", err)
	}
	defer resp.Body.Close()

	var rates DailyRates
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &rates, nil
}
