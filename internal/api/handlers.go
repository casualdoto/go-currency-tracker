package api

import (
	"encoding/json"
	"net/http"

	"github.com/casualdoto/go-currency-tracker/internal/currency"
)

// Middleware для добавления CORS заголовков
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Добавляем CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Обрабатываем preflight запросы
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Передаем запрос следующему обработчику
		next.ServeHTTP(w, r)
	})
}

// Вспомогательная функция для отправки JSON ответа
func sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Ошибка сериализации JSON", http.StatusInternalServerError)
	}
}

// Вспомогательная функция для отправки ошибки
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

// Пример: возврат базовой информации о сервисе
func InfoHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"service": "Go Currency Tracker",
		"status":  "OK",
	}
	sendJSONResponse(w, resp, http.StatusOK)
}

// Обработчик для получения всех курсов валют
func CBRRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем параметр даты из запроса
	date := r.URL.Query().Get("date")

	var rates *currency.DailyRates
	var err error

	// Получаем курсы за указанную дату или текущую, если дата не указана
	rates, err = currency.GetCBRRatesByDate(date)
	if err != nil {
		sendErrorResponse(w, "Ошибка получения курсов: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, rates.Valute, http.StatusOK)
}

// Обработчик для получения курса конкретной валюты
func CBRCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем код валюты из URL
	code := r.URL.Query().Get("code")
	if code == "" {
		sendErrorResponse(w, "Не указан код валюты", http.StatusBadRequest)
		return
	}

	// Получаем параметр даты из запроса
	date := r.URL.Query().Get("date")

	// Получаем курс валюты
	valute, err := currency.GetCurrencyRate(code, date)
	if err != nil {
		sendErrorResponse(w, "Ошибка получения курса валюты: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, valute, http.StatusOK)
}
