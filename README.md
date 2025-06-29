# Go Currency Tracker

A service for tracking currency exchange rates from the Central Bank of Russia (CBR) with REST API.

## Features

- Get all currency rates from CBR
- Get specific currency rate by code
- Select date for historical rates
- OpenAPI documentation
- PostgreSQL database for storing historical rates
- Automatic daily updates at 23:59 UTC

## Installation and Running

### Requirements

- Go 1.21+
- Docker and Docker Compose (for PostgreSQL)

### Building and Running

```bash
# Clone repository
git clone https://github.com/casualdoto/go-currency-tracker.git
cd go-currency-tracker

# Start PostgreSQL database
docker-compose up -d postgres

# Build project
go build -o currency-tracker ./cmd/server

# Run server
./currency-tracker
```

On Windows:

```cmd
go build -o currency-tracker.exe ./cmd/server
currency-tracker.exe
```

## API

Server starts on port 8080 by default.

### Main endpoints

- `GET /ping` - API health check
- `GET /info` - Service information
- `GET /rates/cbr` - Get all currency rates
- `GET /rates/cbr?date=YYYY-MM-DD` - Get all currency rates for specific date
- `GET /rates/cbr/currency?code=USD` - Get USD rate
- `GET /rates/cbr/currency?code=EUR&date=2023-05-15` - Get EUR rate for May 15, 2023
- `GET /api/docs` - OpenAPI documentation

### Request examples

```bash
# Get all currency rates for current date
curl http://localhost:8080/rates/cbr

# Get USD rate for current date
curl http://localhost:8080/rates/cbr/currency?code=USD

# Get EUR rate for specific date
curl http://localhost:8080/rates/cbr/currency?code=EUR&date=2023-05-15
```

## Database

The application uses PostgreSQL to store historical currency rates. The database is automatically updated every day at 23:59 UTC.

### Database Schema

```sql
CREATE TABLE currency_rates (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    currency_code VARCHAR(3) NOT NULL,
    currency_name VARCHAR(100) NOT NULL,
    nominal INTEGER NOT NULL,
    value DECIMAL(12, 4) NOT NULL,
    previous DECIMAL(12, 4),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(date, currency_code)
);
```

### Environment Variables

You can configure the database connection using environment variables:

- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_USER` - Database user (default: currency_user)
- `DB_PASSWORD` - Database password (default: currency_password)
- `DB_NAME` - Database name (default: currency_db)
- `DB_SSLMODE` - SSL mode (default: disable)

## Testing

The project includes comprehensive tests for both API handlers and currency rate functions:

### Running Tests

Run all tests:
```bash
go test ./...
```

Run specific package tests:
```bash
go test ./internal/api -v
go test ./internal/currency -v
```

### Test Coverage

The tests cover:
- API handlers with mock currency providers
- Currency rate functions with mock HTTP servers
- CORS middleware
- Error handling scenarios

## API Documentation

OpenAPI documentation is available at `/api/docs` after starting the server.

## License

MIT
