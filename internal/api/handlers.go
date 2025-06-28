package api

import (
	"encoding/json"
	"net/http"
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
