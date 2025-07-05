# Go Currency Tracker

A service for tracking currency exchange rates from the Central Bank of Russia (CBR) with REST API, web interface for analysis, and Telegram bot for daily updates.

## Features

- Get all currency rates from CBR
- Get specific currency rate by code
- Select date for historical rates
- Web interface for currency analysis with metrics and charts
- Telegram bot for daily currency rate updates
- OpenAPI documentation
- PostgreSQL database for storing historical rates
- Automatic daily updates at 23:59 UTC

## Installation and Running

### Requirements

- Docker and Docker Compose 
- Telegram Bot Token 
- Go is optional (if you want to build or run the app outside of Docker)

### Building and Running

```bash
# Clone repository
git clone https://github.com/casualdoto/go-currency-tracker.git
cd go-currency-tracker

# Copy environment example and set your Telegram Bot Token
cp configs/.env.example .env
# Edit .env file and set your TELEGRAM_BOT_TOKEN

# Start all services using Docker Compose
docker-compose up -d

#Rebuild and start
docker-compose up -d --build

# Or start services individually

# Start PostgreSQL database
docker-compose up -d postgres

# Build and run web server
go build -o currency-tracker.exe ./cmd/server
./currency-tracker

# Build and run Telegram bot
go build -o currency-bot.exe ./cmd/bot
./currency-bot
```

## Web Interface

The application includes a web interface for analyzing currency rates. Access it by opening `http://localhost:8080/` in your browser.

### Features of the Web Interface

- Select any currency from the list of available currencies
- Choose analysis period (1 week, 2 weeks, 1 month, 6 months, 1 year)
- Custom date range selection for precise analysis (up to 365 days)
- View key metrics:
  - Average value
  - Standard deviation
  - Minimum and maximum values
  - Volatility percentage
- Interactive chart showing currency rate changes over time
- Link to subscribe to the Telegram bot for daily updates

## Telegram Bot

The application includes a Telegram bot that provides daily currency rate updates.

### Bot Features

- Subscribe to multiple currencies for daily updates
- Get instant currency rates on demand
- Compare current rates with previous day rates
- View percentage changes in currency rates

### Bot Commands

- `/start` - Start the bot and see available commands
- `/subscribe [currency]` - Subscribe to a currency (e.g., `/subscribe USD`)
- `/unsubscribe [currency]` - Unsubscribe from a currency (e.g., `/unsubscribe USD`)
- `/list` - List your subscriptions
- `/rate [currency]` - Get current rate for a currency (e.g., `/rate USD`)

### Setting Up the Bot

1. Create a new bot with [@BotFather](https://t.me/BotFather) on Telegram
2. Get your bot token
3. Set the token in the `.env` file:
   ```
   TELEGRAM_BOT_TOKEN=your_bot_token_here
   ```
4. Start the bot service using Docker Compose or directly

## API

Server starts on port 8080 by default.

### Main endpoints

- `GET /` - Web interface for currency analysis
- `GET /ping` - API health check
- `GET /info` - Service information
- `GET /rates/cbr` - Get all currency rates
- `GET /rates/cbr?date=YYYY-MM-DD` - Get all currency rates for specific date
- `GET /rates/cbr/currency?code=USD` - Get USD rate
- `GET /rates/cbr/currency?code=EUR&date=2023-05-15` - Get EUR rate for May 15, 2023
- `GET /rates/cbr/history?code=USD&days=30` - Get USD rate history for the last 30 days
- `GET /rates/cbr/history/range?code=USD&start_date=2023-01-01&end_date=2023-01-31` - Get USD rate history for custom date range
- `GET /api/docs` - OpenAPI documentation

### Request examples

```bash
# Get all currency rates for current date
curl http://localhost:8080/rates/cbr

# Get USD rate for current date
curl http://localhost:8080/rates/cbr/currency?code=USD

# Get EUR rate for specific date
curl http://localhost:8080/rates/cbr/currency?code=EUR&date=2023-05-15

# Get USD rate history for the last 30 days
curl http://localhost:8080/rates/cbr/history?code=USD&days=30

# Get USD rate history for custom date range
curl http://localhost:8080/rates/cbr/history/range?code=USD&start_date=2023-01-01&end_date=2023-01-31
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

You can configure the application using environment variables:

- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_USER` - Database user (default: currency_user)
- `DB_PASSWORD` - Database password (default: currency_password)
- `DB_NAME` - Database name (default: currency_db)
- `DB_SSLMODE` - SSL mode (default: disable)
- `TELEGRAM_BOT_TOKEN` - Telegram Bot API token (required for bot functionality)

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
