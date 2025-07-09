# Go Currency Tracker

A service for tracking currency exchange rates from the Central Bank of Russia (CBR) and cryptocurrency rates from Binance with REST API, web interface for analysis, and Telegram bot for daily updates.

## Features

- Get all currency rates from CBR
- Get specific currency rate by code
- Get cryptocurrency rates from Binance converted to RUB
- Select date for historical rates
- Web interface for currency analysis with metrics and charts
- Telegram bot for daily currency rate updates and crypto monitoring
- OpenAPI documentation
- PostgreSQL database for storing historical rates
- Automatic daily updates at 23:59 UTC
- 15-minute crypto rate monitoring with intelligent notifications

## How Currency Conversion Works

### CBR Currencies
- Direct rates from Central Bank of Russia
- Updated daily at official exchange rates
- Historical data available for any date

### Cryptocurrency Rates
- Crypto prices fetched from Binance in USDT
- USD/RUB rate fetched from CBR (current or historical)
- Conversion: **crypto/USDT × USD/RUB = crypto/RUB**
- For current rates: uses today's CBR USD rate
- For historical rates: uses CBR USD rate for specific date
- USDT is treated as equivalent to USD for conversion purposes

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

- **Data Source Selection**: Choose between CBR currencies or cryptocurrencies
- **Currency/Crypto Selection**: Select from available currencies or popular cryptocurrencies
- **Analysis Periods**: Choose from 1 week, 2 weeks, 1 month, 6 months, 1 year
- **Custom Date Range**: Select precise analysis period (up to 365 days)
- **Key Metrics**:
  - Average value
  - Standard deviation
  - Minimum and maximum values
  - Volatility percentage
- **Interactive Charts**: View rate changes over time with different styling for currencies vs crypto
- **Excel Export**: Download historical data as Excel files
- **Telegram Bot Integration**: Direct link to subscribe for updates

## Telegram Bot

The application includes a Telegram bot that provides daily currency rate updates and real-time cryptocurrency monitoring.

### Bot Features

- Subscribe to multiple currencies for daily updates
- Subscribe to cryptocurrencies for 15-minute monitoring
- Get instant currency and crypto rates on demand
- Compare current rates with previous day rates
- View percentage changes in currency rates
- Smart crypto notifications (only for significant changes >= 2%)

### Bot Commands

#### General Commands
- `/start` - Start the bot and see available commands

#### Currency Commands
- `/currencies` - Get list of available currencies
- `/subscribe [currency]` - Subscribe to a currency (e.g., `/subscribe USD`)
- `/unsubscribe [currency]` - Unsubscribe from a currency (e.g., `/unsubscribe USD`)
- `/list` - List your currency subscriptions
- `/rate [currency]` - Get current rate for a currency (e.g., `/rate USD`)

#### Cryptocurrency Commands
- `/cryptocurrencies` - Get list of available cryptocurrencies
- `/crypto_subscribe [symbol]` - Subscribe to crypto updates (e.g., `/crypto_subscribe BTC`)
- `/crypto_unsubscribe [symbol]` - Unsubscribe from crypto updates (e.g., `/crypto_unsubscribe BTC`)
- `/crypto_list` - List your crypto subscriptions
- `/crypto_rate [symbol]` - Get current rate for a cryptocurrency (e.g., `/crypto_rate BTC`)

### Notification Schedule

- **Daily Updates**: 2:00 UTC - All subscribed currencies and cryptocurrencies
- **Crypto Updates**: Every 15 minutes - Only for significant price changes (>= 2%)

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

#### Web Interface
- `GET /` - Web interface for currency analysis

#### General
- `GET /ping` - API health check
- `GET /info` - Service information
- `GET /api/docs` - OpenAPI documentation

#### CBR Currency Endpoints
- `GET /rates/cbr` - Get all currency rates
- `GET /rates/cbr?date=YYYY-MM-DD` - Get all currency rates for specific date
- `GET /rates/cbr/currency?code=USD` - Get USD rate
- `GET /rates/cbr/currency?code=EUR&date=2023-05-15` - Get EUR rate for May 15, 2023
- `GET /rates/cbr/history?code=USD&days=30` - Get USD rate history for the last 30 days
- `GET /rates/cbr/history/range?code=USD&start_date=2023-01-01&end_date=2023-01-31` - Get USD rate history for custom date range
- `GET /rates/cbr/history/range/excel?code=USD&start_date=2023-01-01&end_date=2023-01-31` - Export USD rate history to Excel

#### Cryptocurrency Endpoints
- `GET /rates/crypto/symbols` - Get list of available cryptocurrency symbols
- `GET /rates/crypto/history?symbol=BTC&days=30` - Get cryptocurrency rate history for the last 30 days
- `GET /rates/crypto/history/range?symbol=BTC&start_date=2023-01-01&end_date=2023-01-31` - Get cryptocurrency rate history for custom date range
- `GET /rates/crypto/history/range/excel?symbol=BTC&start_date=2023-01-01&end_date=2023-01-31` - Export cryptocurrency rate history to Excel

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

# Get list of available cryptocurrency symbols
curl http://localhost:8080/rates/crypto/symbols

# Get BTC rate history for the last 30 days (converted to RUB)
curl http://localhost:8080/rates/crypto/history?symbol=BTC&days=30

# Get BTC rate history for custom date range
curl http://localhost:8080/rates/crypto/history/range?symbol=BTC&start_date=2023-01-01&end_date=2023-01-31

# Export BTC rate history to Excel
curl -o btc_history.xlsx http://localhost:8080/rates/crypto/history/range/excel?symbol=BTC&start_date=2023-01-01&end_date=2023-01-31
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

CREATE TABLE crypto_rates (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    price DECIMAL(24, 8) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(date, symbol)
);

CREATE TABLE telegram_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    currency_code VARCHAR(3) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, currency_code)
);

CREATE TABLE telegram_crypto_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, symbol)
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
- `CBR_BASE_URL` - CBR API base URL (default: https://www.cbr-xml-daily.ru)

## Supported Cryptocurrencies

The application supports the following popular cryptocurrencies:

- BTC (Bitcoin)
- ETH (Ethereum)
- BNB (Binance Coin)
- SOL (Solana)
- XRP (XRP)
- ADA (Cardano)
- AVAX (Avalanche)
- DOT (Polkadot)
- DOGE (Dogecoin)
- SHIB (Shiba Inu)
- LINK (Chainlink)
- MATIC (Polygon)
- UNI (Uniswap)
- LTC (Litecoin)
- ATOM (Cosmos)
- XTZ (Tezos)
- FIL (Filecoin)
- TRX (TRON)
- ETC (Ethereum Classic)
- NEAR (NEAR Protocol)

## Testing

The project includes comprehensive tests for both API handlers and currency rate functions:

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test -v ./internal/api

# Run tests with coverage
go test -cover ./...
```

## API Documentation

OpenAPI documentation is available at `/api/docs` after starting the server.

## Architecture

The application follows a clean architecture pattern:

```
go-currency-tracker/
├── cmd/                    # Application entry points
│   ├── bot/               # Telegram bot service
│   └── server/            # Web server service
├── internal/
│   ├── api/               # HTTP handlers and routes
│   ├── currency/          # Currency rate providers
│   │   ├── cbr/          # CBR API integration
│   │   └── binance/      # Binance API integration
│   ├── storage/          # Database layer
│   ├── scheduler/        # Background jobs
│   └── alert/            # Telegram bot implementation
├── web/                  # Frontend assets
└── configs/             # Configuration files
```

## License

MIT
