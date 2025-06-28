package api

import (
	"encoding/json"
	"net/http"

	"github.com/casualdoto/go-currency-tracker/internal/currency"
)

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

// Пример: возврат базовой информации о сервисе
func InfoHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"service": "Go Currency Tracker",
		"status":  "OK",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func CBRRatesHandler(w http.ResponseWriter, r *http.Request) {
	rates, err := currency.GetCBRRates()
	if err != nil {
		http.Error(w, "Ошибка получения курсов: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rates.Valute)
}
