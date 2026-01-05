# Currency Tracker - Microservices Architecture

This directory contains the microservices implementation of the currency tracking system using event-driven architecture with Kafka, Redis caching, and PostgreSQL persistence.

## Architecture Overview

```
┌─────────────┐    ┌─────────────┐
│   Web UI    │    │ Telegram Bot│
└──────┬──────┘    └──────┬──────┘
       │                  │
       └────────┬─────────┘
                │ HTTP
        ┌───────▼────────┐
        │  API Gateway   │
        │  (auth/routing)│
        └───────┬────────┘
                │
        ┌───────▼────────┐
        │ Rates Service  │
        │ (core API)     │
        └───────┬────────┘
                │
    ┌───────────┼───────────┐
    │           │           │
┌───▼───┐  ┌───▼────┐  ┌───▼────┐
│Redis  │  │Postgres│  │ Kafka  │
│(cache)│  │(source)│  │(events)│
└───────┘  └────────┘  └───┬────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼──────┐  ┌────────▼─────────┐  ┌─────▼────────┐
│  Scheduler   │  │   CBR Worker     │  │Binance Worker│
│  (cron)      │  │  (CBR API)       │  │(Binance API) │
└──────────────┘  └──────────────────┘  └──────────────┘
```

## Services

### API Gateway (nginx)
- Single entry point for all external requests using nginx
- Request routing with rate limiting (10 req/sec) and load balancing
- CORS handling, security headers (X-Frame-Options, CSP, etc.), and gzip compression
- Reverse proxy to rates-service with upstream configuration
- DNS resolver for dynamic Docker container resolution

### Rates Service (`rates-service/`)
- Core business logic and API endpoints
- PostgreSQL database operations
- Redis cache management (Cache-aside pattern)
- Kafka event consumption and production

### Scheduler Worker (`scheduler-worker/`)
- Cron-based job scheduling
- Publishes `rates.refresh.requested` events to Kafka

### CBR Worker (`cbr-worker/`)
- Consumes refresh events from Kafka
- Fetches currency rates from Central Bank of Russia API
- Publishes normalized data as `rates.source.updated` events

### Binance Worker (`binance-worker/`)
- Consumes refresh events from Kafka
- Fetches cryptocurrency rates from Binance API
- Publishes normalized data as `rates.source.updated` events

## Infrastructure

### PostgreSQL
- Primary data storage
- Stores historical and current rates
- ACID transactions

### Redis
- High-performance caching layer
- Cache-aside pattern implementation
- TTL-based cache expiration

### Kafka
- Event-driven communication between services
- Topics:
  - `rates.refresh.requested` - Rate update requests
  - `rates.source.updated` - New rates from external APIs
  - `rates.updated` - Confirmation of persisted rates

### Zookeeper
- Kafka cluster coordination

## Getting Started

1. **Prerequisites**
   - Docker and Docker Compose
   - Go 1.21+ (for local development)

2. **Start Infrastructure**
   ```bash
   cd microservices
   docker-compose up -d
   ```

3. **Build Services** (optional, for local development)
   ```bash
   # Build all services
   for service in api-gateway rates-service scheduler-worker cbr-worker binance-worker; do
     cd $service && go build -o main . && cd ..
   done
   ```

4. **Environment Variables**
   Copy `docker/env-example.txt` to your environment configuration.

## Development

### Project Structure
```
microservices/
├── shared/                    # Shared types, config, events
│   ├── config/
│   ├── events/
│   └── types/
├── rates-service/            # Core rates service
├── scheduler-worker/         # Cron scheduler
├── cbr-worker/               # CBR API worker
├── binance-worker/           # Binance API worker
├── docker/                   # Docker configurations
│   ├── nginx/
│   │   └── nginx.conf        # API Gateway configuration
│   ├── postgres/
│   │   └── init.sql          # Database schema
│   └── env-example.txt       # Environment variables
├── docker-compose.yml        # Infrastructure orchestration
└── README.md
```

### Adding New Features

1. **New Event Types**: Add to `shared/events/events.go`
2. **New Data Types**: Add to `shared/types/types.go`
3. **Configuration**: Update `shared/config/config.go`

## API Endpoints

### API Gateway (nginx, port 8080)
- `GET /` - API Gateway info
- `GET /health` - Health check
- `GET /api/*` - All API routes proxied to rates-service with:
  - Rate limiting (10 req/sec)
  - CORS headers
  - Security headers
  - Gzip compression

### Rates Service (port 8081, internal)
- `GET /health` - Service health check
- `GET /rates` - Get current rates (TODO)
- `GET /rates/{source}` - Get rates by source (TODO)

## Monitoring

- Health checks: `/health` endpoint on each service
- Logs: Structured JSON logging with correlation IDs
- Metrics: Prometheus endpoints (TODO)

## Next Steps

See the main project [TODO list](../README.md#microservices-architecture-tasks) for upcoming features and improvements.
