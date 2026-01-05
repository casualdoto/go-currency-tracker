-- Currency Tracker Database Schema

-- Create database if it doesn't exist
CREATE DATABASE IF NOT EXISTS currency_tracker;

-- Use the database
\c currency_tracker;

-- Currency rates table
CREATE TABLE IF NOT EXISTS currency_rates (
    id SERIAL PRIMARY KEY,
    source VARCHAR(50) NOT NULL, -- 'cbr' or 'binance'
    currency_code VARCHAR(10) NOT NULL,
    currency_name VARCHAR(100),
    rate DECIMAL(20, 10) NOT NULL,
    base_currency VARCHAR(10) DEFAULT 'RUB',
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_currency_rates_source_timestamp ON currency_rates(source, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_currency_rates_currency_code ON currency_rates(currency_code);
CREATE INDEX IF NOT EXISTS idx_currency_rates_timestamp ON currency_rates(timestamp DESC);

-- Cryptocurrency rates table (for Binance)
CREATE TABLE IF NOT EXISTS crypto_rates (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL, -- e.g., 'BTCUSDT'
    base_asset VARCHAR(10) NOT NULL, -- e.g., 'BTC'
    quote_asset VARCHAR(10) NOT NULL, -- e.g., 'USDT'
    price DECIMAL(30, 15) NOT NULL,
    volume DECIMAL(30, 8),
    price_change_percent DECIMAL(10, 4),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for crypto rates
CREATE INDEX IF NOT EXISTS idx_crypto_rates_symbol_timestamp ON crypto_rates(symbol, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_crypto_rates_timestamp ON crypto_rates(timestamp DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_currency_rates_updated_at BEFORE UPDATE ON currency_rates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_crypto_rates_updated_at BEFORE UPDATE ON crypto_rates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
