package main

import (
	"log"
	"net/http"
	"os"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := getEnv("SERVER_PORT", "3000")

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	addr := ":" + port
	log.Printf("Web UI Service listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
