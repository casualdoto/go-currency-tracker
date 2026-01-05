# Go Currency Tracker

A service for tracking currency exchange rates from the Central Bank of Russia (CBR) and cryptocurrency rates from Binance with REST API, web interface for analysis, and Telegram bot for daily updates.

## Project Structure

This repository contains two implementations of the currency tracking system:

- **`monolith/`** - Monolithic architecture implementation
- **`microservices/`** - Microservices architecture implementation (event-driven)

## Monolith Architecture

The `monolith/` directory contains a traditional monolithic application where all components are tightly coupled and run as a single deployable unit.

### Components

- **Web Server** (`cmd/server/`) - REST API server with web interface
- **Telegram Bot** (`cmd/bot/`) - Telegram bot service for notifications
- **API Layer** (`internal/api/`) - HTTP handlers and routes
- **Currency Providers** (`internal/currency/`) - CBR and Binance API integrations
- **Storage** (`internal/storage/`) - PostgreSQL database layer
- **Scheduler** (`internal/scheduler/`) - Background job scheduling
- **Alert System** (`internal/alert/`) - Telegram bot implementation

### Architecture Pattern

- **Clean Architecture** with layered structure
- **Synchronous processing** - all operations happen in real-time
- **Direct database access** - no caching layer
- **In-process scheduling** - cron jobs run within application processes

### Use Cases

- Simple deployment and development
- Lower operational complexity
- Suitable for small to medium scale applications
- Single database connection pool

For detailed documentation, see [monolith/README.md](monolith/README.md).

## Microservices Architecture

The `microservices/` directory contains an event-driven microservices implementation with asynchronous processing, caching, and message queue integration.

### Architecture Overview

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

### Components

#### **API Gateway**
- Single entry point for all external requests
- Authentication and authorization
- Request routing to appropriate services
- Rate limiting and request throttling

#### **Rates Service (Core API)**
- Domain logic and business rules
- REST API endpoints for currency and crypto rates
- Cache management (Redis with Cache-aside pattern)
- Database operations (PostgreSQL as source of truth)
- Event publishing to Kafka

#### **Data Storage**

- **PostgreSQL** - Primary database (source of truth)
  - Stores all historical and current rates
  - ACID transactions
  - Read/Write operations

- **Redis** - Caching layer
  - Cache-aside pattern
  - TTL-based expiration
  - Fast read access for frequently requested data

- **Kafka** - Event bus
  - Asynchronous event-driven communication
  - Topics for different event types
  - Decoupled service communication

#### **Workers (Background Services)**

- **Scheduler Worker**
  - Cron-based job scheduling
  - Publishes `rates.refresh.requested` events
  - Triggers periodic data updates

- **CBR Worker**
  - Consumes refresh events from Kafka
  - Fetches currency rates from CBR API
  - Normalizes and publishes `rates.source.updated` events

- **Binance Worker**
  - Consumes refresh events from Kafka
  - Fetches cryptocurrency rates from Binance API
  - Normalizes and publishes `rates.source.updated` events

### How It Works

#### 1️⃣ Reading Data (Web / Telegram)

```
Web UI / Telegram Bot
    ↓ HTTP
API Gateway
    ↓
Rates Service:
    1. Check Redis cache
    2. If cache miss → Read from Postgres
    3. Store in Redis (with TTL)
    4. Return response to client
```

**Flow:**
- Client request → API Gateway (auth, routing, rate limit)
- Gateway → Rates Service
- Rates Service checks Redis first
- Cache miss → Query Postgres
- Update Redis cache
- Return response

#### 2️⃣ Updating Rates (Asynchronously)

```
Scheduler (cron)
    ↓
Publishes: rates.refresh.requested
    ↓
┌─────────────────┬─────────────────┐
│                 │                 │
CBR Worker    Binance Worker
│                 │
Consume event  Consume event
│                 │
Fetch CBR API  Fetch Binance API
│                 │
Normalize data  Normalize data
│                 │
Publish: rates.source.updated
    ↓
Rates Service:
    1. Consumes rates.source.updated
    2. Updates Postgres
    3. Invalidates/updates Redis
    4. Publishes: rates.updated
```

**Event Flow:**
1. **Scheduler** publishes `rates.refresh.requested` to Kafka
2. **CBR Worker** and **Binance Worker** consume the event
3. Workers fetch data from external APIs
4. Workers normalize and publish `rates.source.updated`
5. **Rates Service** consumes `rates.source.updated`
6. Service updates Postgres and Redis
7. Service publishes `rates.updated` (for other subscribers)

### Kafka Topics

- `rates.refresh.requested` - Request to refresh rates from external sources
- `rates.source.updated` - New rates received from external APIs
- `rates.updated` - Rates successfully persisted and cached

### Benefits of Microservices Architecture

- **Scalability** - Independent scaling of components
- **Resilience** - Service failures don't cascade
- **Performance** - Redis caching reduces database load
- **Asynchronous Processing** - Non-blocking updates
- **Decoupling** - Services communicate via events
- **Technology Flexibility** - Different services can use different tech stacks

### Use Cases

- High traffic applications
- Need for horizontal scaling
- Complex business logic requiring separation
- Multiple teams working on different services
- Real-time data processing requirements

## Comparison

| Aspect | Monolith | Microservices |
|--------|----------|---------------|
| **Deployment** | Single unit | Multiple services |
| **Complexity** | Lower | Higher |
| **Scalability** | Vertical | Horizontal |
| **Caching** | None | Redis |
| **Processing** | Synchronous | Asynchronous (Kafka) |
| **Development** | Simpler | More complex |
| **Operations** | Easier | More complex |

## Getting Started

Choose the architecture that fits your needs:

- For **simple deployments** and **small scale** → Use `monolith/`
- For **production scale** and **high availability** → Use `microservices/`

See respective README files in each directory for detailed setup instructions.

## TODO List

### Project-Wide Tasks

1. **Documentation**
   - [x] Create root README with architecture comparison
   - [ ] Add architectural diagrams for both implementations
   - [ ] Add changelog for versions
   - [ ] Create deployment guides for both architectures

2. **CI/CD**
   - [ ] Add CI/CD pipeline with GitHub Actions
   - [ ] Configure automated testing in containers
   - [ ] Add Docker health checks
   - [ ] Optimize Docker image sizes

### Monolith Architecture Tasks

For detailed TODO list, see [monolith/README.md](monolith/README.md#todo-list).

**High Priority:**
- [ ] Centralized logging (structured logging with `slog`)
- [ ] Configuration improvements and validation
- [ ] Input data validation

**Medium Priority:**
- [ ] Performance optimization
- [ ] Security improvements
- [ ] Extended testing

### Microservices Architecture Tasks

**High Priority:**

1. **Core Infrastructure**
   - [x] Implement API Gateway service (nginx with rate limiting, CORS, security headers)
   - [x] Set up Kafka cluster and topics (Kafka + Zookeeper, auto-created topics)
   - [x] Configure Redis for caching (Redis ready for Cache-aside pattern)
   - [x] Create Docker Compose for all services (full orchestration with networking)

2. **Rates Service**
   - [ ] Implement core API service with domain logic
   - [ ] Add Redis cache integration (Cache-aside pattern)
   - [ ] Implement Kafka consumer for `rates.source.updated`
   - [ ] Implement Kafka producer for `rates.updated`
   - [x] Add health checks and metrics (HTTP health endpoint implemented)

3. **Workers**
   - [x] Implement Scheduler Worker with cron (runs every 5 minutes, publishes refresh events)
   - [x] Implement CBR Worker (Kafka consumer structure ready, TODO: CBR API integration)
   - [x] Implement Binance Worker (Kafka consumer structure ready, TODO: Binance API integration)
   - [ ] Add error handling and retry mechanisms
   - [ ] Implement data normalization logic

**Medium Priority:**

4. **Event System**
   - [x] Define Kafka event schemas (Event types with JSON serialization)
   - [x] Implement event serialization/deserialization (JSON marshaling/unmarshaling)
   - [ ] Add event versioning support
   - [ ] Implement dead letter queue for failed events

5. **Caching Strategy**
   - [ ] Design cache key structure
   - [ ] Implement cache invalidation on updates
   - [ ] Add cache warming strategies
   - [ ] Configure TTL policies

6. **Monitoring and Observability**
   - [ ] Add Prometheus metrics for all services
   - [ ] Implement distributed tracing (OpenTelemetry)
   - [x] Add structured logging with correlation IDs (JSON logging implemented)
   - [ ] Configure alerts for critical metrics

**Low Priority:**

7. **Resilience**
   - [ ] Implement circuit breaker for external APIs
   - [ ] Add retry policies with exponential backoff
   - [ ] Implement rate limiting per service
   - [ ] Add graceful degradation

8. **Testing**
   - [ ] Unit tests for all services
   - [ ] Integration tests with TestContainers
   - [ ] End-to-end tests for event flows
   - [ ] Load testing for scalability

9. **Security**
   - [ ] Implement authentication in API Gateway
   - [ ] Add API key management
   - [ ] Configure service-to-service authentication
   - [x] Add input validation and sanitization (nginx security headers, CORS, rate limiting)

10. **DevOps**
    - [ ] Kubernetes deployment manifests
    - [ ] Helm charts for easy deployment
    - [ ] Service mesh integration (optional)
    - [ ] Multi-environment configuration

## Infrastructure Status

✅ **Fully implemented and tested:**
- Docker Compose orchestration with all services
- Nginx API Gateway with rate limiting, CORS, and security headers
- Kafka event-driven communication with auto-created topics
- Redis caching infrastructure ready
- PostgreSQL with initialized schema
- Health checks for all services
- Structured JSON logging
- Shared configuration and event system

**Next focus areas:**
- Business logic implementation in Rates Service (API endpoints, Redis cache, Kafka consumers)
- External API integrations in Workers (CBR and Binance API clients)
- Data normalization and error handling

*This TODO list will be updated as tasks are completed and new improvements are identified.*
