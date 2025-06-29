# Go Currency Tracker

A service for tracking currency exchange rates from the Central Bank of Russia (CBR) with REST API.

## Features

- Get all currency rates from CBR
- Get specific currency rate by code
- Select date for historical rates
- OpenAPI documentation

## Installation and Running

### Requirements

- Go 1.21+

### Building and Running

```bash
# Clone repository
git clone https://github.com/casualdoto/go-currency-tracker.git
cd go-currency-tracker

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
