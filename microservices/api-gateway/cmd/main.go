package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/casualdoto/go-currency-tracker/microservices/api-gateway/internal/config"
	"github.com/casualdoto/go-currency-tracker/microservices/api-gateway/internal/gateway"
)

func main() {
	cfg := config.Load()

	gw := gateway.New(cfg)

	addr := ":" + cfg.ServerPort
	log.Printf("API Gateway listening on %s", addr)

	srv := &http.Server{Addr: addr, Handler: gw.Routes()}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("API Gateway: shutting down")
}
