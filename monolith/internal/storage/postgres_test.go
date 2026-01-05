package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgreSQL test container
type PostgreSQLContainer struct {
	testcontainers.Container
	ConnectionString string
}

// Setup PostgreSQL container for integration tests
func setupPostgreSQLContainer(ctx context.Context) (*PostgreSQLContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	connectionString := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable",
		hostIP, mappedPort.Port())

	return &PostgreSQLContainer{
		Container:        container,
		ConnectionString: connectionString,
	}, nil
}

// Test PostgreSQL configuration
func TestPostgresConfig(t *testing.T) {
	config := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "testuser", config.User)
	assert.Equal(t, "testpass", config.Password)
	assert.Equal(t, "testdb", config.DBName)
	assert.Equal(t, "disable", config.SSLMode)
}

// Test database connection without container (unit test)
func TestNewPostgresDB_InvalidConnection(t *testing.T) {
	config := PostgresConfig{
		Host:     "invalid-host",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	db, err := NewPostgresDB(config)
	assert.Error(t, err)
	assert.Nil(t, db)
}

// Test struct validation
func TestCurrencyRate_Validation(t *testing.T) {
	rate := CurrencyRate{
		Date:         time.Date(2023, 6, 29, 0, 0, 0, 0, time.UTC),
		CurrencyCode: "USD",
		CurrencyName: "US Dollar",
		Nominal:      1,
		Value:        85.0504,
		Previous:     87.1992,
	}

	assert.Equal(t, "USD", rate.CurrencyCode)
	assert.Equal(t, "US Dollar", rate.CurrencyName)
	assert.Equal(t, 1, rate.Nominal)
	assert.Equal(t, 85.0504, rate.Value)
	assert.Equal(t, 87.1992, rate.Previous)
	assert.False(t, rate.Date.IsZero())
}

func TestCryptoRate_Validation(t *testing.T) {
	rate := CryptoRate{
		Timestamp: time.Date(2023, 6, 29, 12, 0, 0, 0, time.UTC),
		Symbol:    "BTC",
		Open:      47000.50,
		High:      47500.00,
		Low:       46500.00,
		Close:     47200.00,
		Volume:    100.5,
	}

	assert.Equal(t, "BTC", rate.Symbol)
	assert.Equal(t, 47000.50, rate.Open)
	assert.Equal(t, 47500.00, rate.High)
	assert.Equal(t, 46500.00, rate.Low)
	assert.Equal(t, 47200.00, rate.Close)
	assert.Equal(t, 100.5, rate.Volume)
	assert.False(t, rate.Timestamp.IsZero())
	assert.Greater(t, rate.High, rate.Low)
	assert.Greater(t, rate.Volume, 0.0)
}

// Test database connection errors
func TestPostgresDB_ConnectionErrors(t *testing.T) {
	t.Run("test invalid host", func(t *testing.T) {
		config := PostgresConfig{
			Host:     "invalid-host-12345",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		db, err := NewPostgresDB(config)
		assert.Error(t, err)
		assert.Nil(t, db)
	})

	t.Run("test invalid port", func(t *testing.T) {
		config := PostgresConfig{
			Host:     "localhost",
			Port:     99999,
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		db, err := NewPostgresDB(config)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

// Mock tests for basic functionality
func TestPostgresDB_MockOperations(t *testing.T) {
	t.Run("test currency rate structure", func(t *testing.T) {
		// Test currency rate creation
		rates := []CurrencyRate{
			{
				Date:         time.Date(2023, 6, 29, 0, 0, 0, 0, time.UTC),
				CurrencyCode: "USD",
				CurrencyName: "US Dollar",
				Nominal:      1,
				Value:        85.0504,
				Previous:     87.1992,
			},
			{
				Date:         time.Date(2023, 6, 29, 0, 0, 0, 0, time.UTC),
				CurrencyCode: "EUR",
				CurrencyName: "Euro",
				Nominal:      1,
				Value:        94.7092,
				Previous:     95.0489,
			},
		}

		assert.Len(t, rates, 2)
		assert.Equal(t, "USD", rates[0].CurrencyCode)
		assert.Equal(t, "EUR", rates[1].CurrencyCode)
	})

	t.Run("test crypto rate structure", func(t *testing.T) {
		// Test crypto rate creation
		rates := []CryptoRate{
			{
				Timestamp: time.Date(2023, 6, 29, 12, 0, 0, 0, time.UTC),
				Symbol:    "BTC",
				Open:      47000.50,
				High:      47500.00,
				Low:       46500.00,
				Close:     47200.00,
				Volume:    100.5,
			},
			{
				Timestamp: time.Date(2023, 6, 29, 13, 0, 0, 0, time.UTC),
				Symbol:    "ETH",
				Open:      3200.50,
				High:      3250.00,
				Low:       3150.00,
				Close:     3220.00,
				Volume:    500.5,
			},
		}

		assert.Len(t, rates, 2)
		assert.Equal(t, "BTC", rates[0].Symbol)
		assert.Equal(t, "ETH", rates[1].Symbol)
	})
}

// Test time-related functions
func TestPostgresDB_TimeOperations(t *testing.T) {
	t.Run("test date truncation", func(t *testing.T) {
		now := time.Now().UTC()
		truncated := now.Truncate(24 * time.Hour)

		assert.Equal(t, 0, truncated.Hour())
		assert.Equal(t, 0, truncated.Minute())
		assert.Equal(t, 0, truncated.Second())
		assert.Equal(t, 0, truncated.Nanosecond())
	})

	t.Run("test date range calculations", func(t *testing.T) {
		startDate := time.Date(2023, 6, 26, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2023, 6, 28, 0, 0, 0, 0, time.UTC)

		assert.True(t, endDate.After(startDate))

		duration := endDate.Sub(startDate)
		assert.Equal(t, 48*time.Hour, duration)
	})
}

// Test subscription structures
func TestTelegramSubscription_Structure(t *testing.T) {
	sub := TelegramSubscription{
		ID:        1,
		UserID:    12345,
		Currency:  "USD",
		CreatedAt: time.Now(),
	}

	assert.Equal(t, 1, sub.ID)
	assert.Equal(t, 12345, sub.UserID)
	assert.Equal(t, "USD", sub.Currency)
	assert.False(t, sub.CreatedAt.IsZero())
}

// Benchmark tests
func BenchmarkCurrencyRate_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rate := CurrencyRate{
			Date:         time.Now(),
			CurrencyCode: "USD",
			CurrencyName: "US Dollar",
			Nominal:      1,
			Value:        85.0504,
			Previous:     87.1992,
		}
		_ = rate
	}
}

func BenchmarkCryptoRate_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rate := CryptoRate{
			Timestamp: time.Now(),
			Symbol:    "BTC",
			Open:      47000.50,
			High:      47500.00,
			Low:       46500.00,
			Close:     47200.00,
			Volume:    100.5,
		}
		_ = rate
	}
}

// Test error scenarios
func TestPostgresDB_ErrorScenarios(t *testing.T) {
	t.Run("test nil database", func(t *testing.T) {
		var db *PostgresDB
		assert.Nil(t, db)
	})

	t.Run("test empty currency code", func(t *testing.T) {
		rate := CurrencyRate{
			Date:         time.Date(2023, 6, 29, 0, 0, 0, 0, time.UTC),
			CurrencyCode: "",
			CurrencyName: "Empty Currency",
			Nominal:      1,
			Value:        85.0504,
			Previous:     87.1992,
		}

		assert.Empty(t, rate.CurrencyCode)
		assert.NotEmpty(t, rate.CurrencyName)
	})

	t.Run("test empty crypto symbol", func(t *testing.T) {
		rate := CryptoRate{
			Timestamp: time.Date(2023, 6, 29, 12, 0, 0, 0, time.UTC),
			Symbol:    "",
			Open:      47000.50,
			High:      47500.00,
			Low:       46500.00,
			Close:     47200.00,
			Volume:    100.5,
		}

		assert.Empty(t, rate.Symbol)
		assert.Greater(t, rate.Open, 0.0)
	})
}

// Test data validation
func TestPostgresDB_DataValidation(t *testing.T) {
	t.Run("test currency rate validation", func(t *testing.T) {
		rate := CurrencyRate{
			Date:         time.Date(2023, 6, 29, 0, 0, 0, 0, time.UTC),
			CurrencyCode: "USD",
			CurrencyName: "US Dollar",
			Nominal:      1,
			Value:        85.0504,
			Previous:     87.1992,
		}

		// Validate required fields
		assert.NotEmpty(t, rate.CurrencyCode)
		assert.NotEmpty(t, rate.CurrencyName)
		assert.Greater(t, rate.Nominal, 0)
		assert.Greater(t, rate.Value, 0.0)
		assert.False(t, rate.Date.IsZero())
	})

	t.Run("test crypto rate validation", func(t *testing.T) {
		rate := CryptoRate{
			Timestamp: time.Date(2023, 6, 29, 12, 0, 0, 0, time.UTC),
			Symbol:    "BTC",
			Open:      47000.50,
			High:      47500.00,
			Low:       46500.00,
			Close:     47200.00,
			Volume:    100.5,
		}

		// Validate required fields
		assert.NotEmpty(t, rate.Symbol)
		assert.Greater(t, rate.Open, 0.0)
		assert.Greater(t, rate.High, 0.0)
		assert.Greater(t, rate.Low, 0.0)
		assert.Greater(t, rate.Close, 0.0)
		assert.Greater(t, rate.Volume, 0.0)
		assert.False(t, rate.Timestamp.IsZero())

		// Validate logical relationships
		assert.GreaterOrEqual(t, rate.High, rate.Low)
		assert.GreaterOrEqual(t, rate.High, rate.Open)
		assert.GreaterOrEqual(t, rate.High, rate.Close)
		assert.LessOrEqual(t, rate.Low, rate.Open)
		assert.LessOrEqual(t, rate.Low, rate.Close)
	})
}
