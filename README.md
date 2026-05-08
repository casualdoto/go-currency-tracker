# Currency Tracker

This is the code for a graduation project at St. Petersburg Polytechnic University on the topic: **"Comparative Analysis of Monolithic and Microservice Architectures Using a Currency and Cryptocurrency Rate Tracking Application."**

## About

Currency Tracker is a Go-based application that fetches fiat currency rates from the Central Bank of Russia (CBR) and cryptocurrency rates from Binance. It provides a REST API, a web interface with interactive charts and statistical metrics, and a Telegram bot for subscription-based rate notifications.

Crypto prices are fetched in USDT from Binance and converted to RUB using the official CBR USD/RUB rate:

> **crypto/USDT x USD/RUB = crypto/RUB**

## Implementations

The repository contains two functionally equivalent implementations for architectural comparison:

| | Description | Link |
|---|---|---|
| **Monolith** | Single-process application (2 entry points sharing one PostgreSQL database) | [`monolith/`](monolith/) |
| **Microservices** | Distributed event-driven architecture with 7 services, Kafka, Redis, PostgreSQL, and ClickHouse | [`microservices/`](microservices/) |

Both implementations offer the same core features:

- Current and historical CBR fiat currency rates
- Current and historical cryptocurrency rates in RUB
- Web interface with Chart.js charts, statistical metrics, and Excel export
- Telegram bot with fiat and crypto subscription notifications
- OpenAPI documentation
- Docker Compose deployment

## Quick Start

### Monolith

```bash
cd monolith
cp configs/.env.example .env   # set TELEGRAM_BOT_TOKEN
docker-compose up --build
```

Web interface: `http://localhost:8081`

### Microservices

```bash
cd microservices
cp configs/.env.example configs/.env   # set TELEGRAM_BOT_TOKEN
docker-compose up --build
```

Web interface: `http://localhost:3000`

## License

MIT
