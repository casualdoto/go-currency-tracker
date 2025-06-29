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

// Получить курсы с сайта ЦБ за текущую дату
func GetCBRRates() (*DailyRates, error) {
	return GetCBRRatesByDate("")
}

// Получить курсы с сайта ЦБ за указанную дату
// Если date пустая строка, возвращает курсы за текущую дату
// Формат даты: YYYY-MM-DD (например, "2023-05-15")
func GetCBRRatesByDate(date string) (*DailyRates, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := "https://www.cbr-xml-daily.ru/daily_json.js"

	// Если указана дата, формируем URL для архива
	if date != "" {
		// Преобразуем формат даты из YYYY-MM-DD в формат для API (YYYY/MM/DD)
		parsedDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
		}

		// Для архивных данных используем другой формат URL
		// В API cbr-xml-daily.ru архивные данные доступны по URL вида:
		// https://www.cbr-xml-daily.ru/archive/YYYY/MM/DD/daily_json.js
		formattedDate := parsedDate.Format("2006/01/02")
		url = fmt.Sprintf("https://www.cbr-xml-daily.ru/archive/%s/daily_json.js", formattedDate)
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CBR rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Если архивные данные не найдены, попробуем получить данные за предыдущий рабочий день
		if date != "" && resp.StatusCode == http.StatusNotFound {
			// Попробуем получить данные за текущую дату
			return GetCBRRates()
		}
		return nil, fmt.Errorf("failed to fetch CBR rates, status code: %d", resp.StatusCode)
	}

	var rates DailyRates
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &rates, nil
}

// Получить курс конкретной валюты
// code - код валюты в формате ISO 4217 (например, USD, EUR)
// date - дата в формате YYYY-MM-DD, если пустая строка - текущая дата
func GetCurrencyRate(code string, date string) (*Valute, error) {
	if code == "" {
		return nil, fmt.Errorf("currency code cannot be empty")
	}

	rates, err := GetCBRRatesByDate(date)
	if err != nil {
		return nil, err
	}

	valute, ok := rates.Valute[code]
	if !ok {
		return nil, fmt.Errorf("currency with code %s not found", code)
	}

	return &valute, nil
}
