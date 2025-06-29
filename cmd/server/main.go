package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/casualdoto/go-currency-tracker/internal/api"
)

func main() {
	// Определяем рабочий каталог для правильного доступа к статическим файлам
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	log.Printf("Working directory: %s", workDir)

	// Проверяем наличие директории api
	apiDir := filepath.Join(workDir, "api")
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		log.Printf("Warning: API docs directory not found at %s", apiDir)
	} else {
		log.Printf("API docs directory found at %s", apiDir)

		// Проверяем наличие файла документации
		apiFile := filepath.Join(apiDir, "openapi.json")
		if _, err := os.Stat(apiFile); os.IsNotExist(err) {
			log.Printf("Warning: API docs file not found at %s", apiFile)
		} else {
			log.Printf("API docs file found at %s", apiFile)
		}
	}

	// Настраиваем маршруты
	router := api.SetupRoutes()

	log.Println("Go Currency Monitor API started on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
