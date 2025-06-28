package main

import (
	"log"
	"net/http"

	"github.com/casualdoto/go-currency-tracker/internal/api"
)

func main() {
	router := api.SetupRoutes()

	log.Println("Go Currency Monitor API started on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
