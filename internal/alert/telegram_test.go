package alert

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock database for testing
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) UpdateSchema() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) GetAllTelegramSubscriptions() (map[int][]string, error) {
	args := m.Called()
	return args.Get(0).(map[int][]string), args.Error(1)
}

func (m *MockDatabase) GetAllTelegramCryptoSubscriptions() (map[int][]string, error) {
	args := m.Called()
	return args.Get(0).(map[int][]string), args.Error(1)
}

func (m *MockDatabase) SaveTelegramSubscription(userID int, currency string) error {
	args := m.Called(userID, currency)
	return args.Error(0)
}

func (m *MockDatabase) DeleteTelegramSubscription(userID int, currency string) error {
	args := m.Called(userID, currency)
	return args.Error(0)
}

func (m *MockDatabase) GetTelegramSubscriptions(userID int) ([]string, error) {
	args := m.Called(userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDatabase) SaveTelegramCryptoSubscription(userID int, symbol string) error {
	args := m.Called(userID, symbol)
	return args.Error(0)
}

func (m *MockDatabase) DeleteTelegramCryptoSubscription(userID int, symbol string) error {
	args := m.Called(userID, symbol)
	return args.Error(0)
}

func (m *MockDatabase) GetTelegramCryptoSubscriptions(userID int) ([]string, error) {
	args := m.Called(userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock Telegram bot for testing
type MockTelegramBot struct {
	bot              interface{}
	subscriptions    map[int][]string
	cryptoSubs       map[int][]string
	lastCryptoPrices map[string]float64
	mu               sync.RWMutex
	db               *MockDatabase
}

func NewMockTelegramBot(db *MockDatabase) *MockTelegramBot {
	return &MockTelegramBot{
		subscriptions:    make(map[int][]string),
		cryptoSubs:       make(map[int][]string),
		lastCryptoPrices: make(map[string]float64),
		mu:               sync.RWMutex{},
		db:               db,
	}
}

func TestSubscription_Structure(t *testing.T) {
	// Test Subscription struct
	sub := Subscription{
		UserID:    12345,
		Currency:  "USD",
		CreatedAt: time.Now(),
	}

	assert.Equal(t, 12345, sub.UserID)
	assert.Equal(t, "USD", sub.Currency)
	assert.False(t, sub.CreatedAt.IsZero())
}

func TestMockTelegramBot_Creation(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	assert.NotNil(t, bot)
	assert.NotNil(t, bot.subscriptions)
	assert.NotNil(t, bot.cryptoSubs)
	assert.NotNil(t, bot.lastCryptoPrices)
	assert.Equal(t, mockDB, bot.db)
}

func TestMockTelegramBot_Subscriptions(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	t.Run("test subscription management", func(t *testing.T) {
		userID := 12345
		currency := "USD"

		// Test adding subscription
		bot.mu.Lock()
		bot.subscriptions[userID] = append(bot.subscriptions[userID], currency)
		bot.mu.Unlock()

		bot.mu.RLock()
		subscriptions, exists := bot.subscriptions[userID]
		bot.mu.RUnlock()

		assert.True(t, exists)
		assert.Contains(t, subscriptions, currency)
		assert.Len(t, subscriptions, 1)
	})

	t.Run("test duplicate subscription prevention", func(t *testing.T) {
		userID := 12345
		currency := "USD"

		bot.mu.Lock()
		// Check if currency already exists
		currencies := bot.subscriptions[userID]
		alreadyExists := false
		for _, c := range currencies {
			if c == currency {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			bot.subscriptions[userID] = append(bot.subscriptions[userID], currency)
		}
		bot.mu.Unlock()

		bot.mu.RLock()
		subscriptions := bot.subscriptions[userID]
		bot.mu.RUnlock()

		// Should still have only one USD subscription
		count := 0
		for _, c := range subscriptions {
			if c == currency {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})
}

func TestMockTelegramBot_CryptoSubscriptions(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	t.Run("test crypto subscription management", func(t *testing.T) {
		userID := 12345
		symbol := "BTC"

		// Test adding crypto subscription
		bot.mu.Lock()
		bot.cryptoSubs[userID] = append(bot.cryptoSubs[userID], symbol)
		bot.mu.Unlock()

		bot.mu.RLock()
		subscriptions, exists := bot.cryptoSubs[userID]
		bot.mu.RUnlock()

		assert.True(t, exists)
		assert.Contains(t, subscriptions, symbol)
		assert.Len(t, subscriptions, 1)
	})

	t.Run("test multiple crypto subscriptions", func(t *testing.T) {
		userID := 54321 // Different userID to avoid conflicts
		symbols := []string{"BTC", "ETH", "BNB"}

		bot.mu.Lock()
		for _, symbol := range symbols {
			bot.cryptoSubs[userID] = append(bot.cryptoSubs[userID], symbol)
		}
		bot.mu.Unlock()

		bot.mu.RLock()
		subscriptions := bot.cryptoSubs[userID]
		bot.mu.RUnlock()

		assert.Len(t, subscriptions, 3)
		for _, symbol := range symbols {
			assert.Contains(t, subscriptions, symbol)
		}
	})
}

func TestMockTelegramBot_LastCryptoPrices(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	t.Run("test crypto price tracking", func(t *testing.T) {
		symbol := "BTC"
		price := 47000.50

		// Test setting last price
		bot.mu.Lock()
		bot.lastCryptoPrices[symbol] = price
		bot.mu.Unlock()

		bot.mu.RLock()
		lastPrice, exists := bot.lastCryptoPrices[symbol]
		bot.mu.RUnlock()

		assert.True(t, exists)
		assert.Equal(t, price, lastPrice)
	})

	t.Run("test multiple crypto prices", func(t *testing.T) {
		prices := map[string]float64{
			"BTC": 47000.50,
			"ETH": 3200.75,
			"BNB": 450.25,
		}

		bot.mu.Lock()
		for symbol, price := range prices {
			bot.lastCryptoPrices[symbol] = price
		}
		bot.mu.Unlock()

		bot.mu.RLock()
		storedPrices := make(map[string]float64)
		for symbol, price := range bot.lastCryptoPrices {
			storedPrices[symbol] = price
		}
		bot.mu.RUnlock()

		for symbol, expectedPrice := range prices {
			actualPrice, exists := storedPrices[symbol]
			assert.True(t, exists)
			assert.Equal(t, expectedPrice, actualPrice)
		}
	})
}

func TestMockTelegramBot_ThreadSafety(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	t.Run("test concurrent access to subscriptions", func(t *testing.T) {
		userID := 12345
		currency := "USD"

		// Test concurrent writes
		done := make(chan bool)
		go func() {
			bot.mu.Lock()
			bot.subscriptions[userID] = append(bot.subscriptions[userID], currency)
			bot.mu.Unlock()
			done <- true
		}()

		go func() {
			bot.mu.RLock()
			_ = bot.subscriptions[userID]
			bot.mu.RUnlock()
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		bot.mu.RLock()
		subscriptions := bot.subscriptions[userID]
		bot.mu.RUnlock()

		assert.Contains(t, subscriptions, currency)
	})
}

func TestMockTelegramBot_UnsubscribeLogic(t *testing.T) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	t.Run("test unsubscribe from currency", func(t *testing.T) {
		userID := 12345
		currencies := []string{"USD", "EUR", "GBP"}

		// Add multiple subscriptions
		bot.mu.Lock()
		bot.subscriptions[userID] = currencies
		bot.mu.Unlock()

		// Remove one subscription
		currencyToRemove := "EUR"
		bot.mu.Lock()
		subscriptions := bot.subscriptions[userID]
		var newSubscriptions []string
		for _, c := range subscriptions {
			if c != currencyToRemove {
				newSubscriptions = append(newSubscriptions, c)
			}
		}
		bot.subscriptions[userID] = newSubscriptions
		bot.mu.Unlock()

		bot.mu.RLock()
		finalSubscriptions := bot.subscriptions[userID]
		bot.mu.RUnlock()

		assert.Len(t, finalSubscriptions, 2)
		assert.Contains(t, finalSubscriptions, "USD")
		assert.Contains(t, finalSubscriptions, "GBP")
		assert.NotContains(t, finalSubscriptions, "EUR")
	})

	t.Run("test unsubscribe from crypto", func(t *testing.T) {
		userID := 12345
		symbols := []string{"BTC", "ETH", "BNB"}

		// Add multiple crypto subscriptions
		bot.mu.Lock()
		bot.cryptoSubs[userID] = symbols
		bot.mu.Unlock()

		// Remove one subscription
		symbolToRemove := "ETH"
		bot.mu.Lock()
		subscriptions := bot.cryptoSubs[userID]
		var newSubscriptions []string
		for _, s := range subscriptions {
			if s != symbolToRemove {
				newSubscriptions = append(newSubscriptions, s)
			}
		}
		bot.cryptoSubs[userID] = newSubscriptions
		bot.mu.Unlock()

		bot.mu.RLock()
		finalSubscriptions := bot.cryptoSubs[userID]
		bot.mu.RUnlock()

		assert.Len(t, finalSubscriptions, 2)
		assert.Contains(t, finalSubscriptions, "BTC")
		assert.Contains(t, finalSubscriptions, "BNB")
		assert.NotContains(t, finalSubscriptions, "ETH")
	})
}

// Test database interactions
func TestMockDatabase_Subscriptions(t *testing.T) {
	mockDB := &MockDatabase{}

	t.Run("test save subscription", func(t *testing.T) {
		userID := 12345
		currency := "USD"

		mockDB.On("SaveTelegramSubscription", userID, currency).Return(nil)

		err := mockDB.SaveTelegramSubscription(userID, currency)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})

	t.Run("test get subscriptions", func(t *testing.T) {
		userID := 12345
		expectedSubscriptions := []string{"USD", "EUR"}

		mockDB.On("GetTelegramSubscriptions", userID).Return(expectedSubscriptions, nil)

		subscriptions, err := mockDB.GetTelegramSubscriptions(userID)
		assert.NoError(t, err)
		assert.Equal(t, expectedSubscriptions, subscriptions)

		mockDB.AssertExpectations(t)
	})

	t.Run("test delete subscription", func(t *testing.T) {
		userID := 12345
		currency := "USD"

		mockDB.On("DeleteTelegramSubscription", userID, currency).Return(nil)

		err := mockDB.DeleteTelegramSubscription(userID, currency)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})
}

func TestMockDatabase_CryptoSubscriptions(t *testing.T) {
	mockDB := &MockDatabase{}

	t.Run("test save crypto subscription", func(t *testing.T) {
		userID := 12345
		symbol := "BTC"

		mockDB.On("SaveTelegramCryptoSubscription", userID, symbol).Return(nil)

		err := mockDB.SaveTelegramCryptoSubscription(userID, symbol)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})

	t.Run("test get crypto subscriptions", func(t *testing.T) {
		userID := 12345
		expectedSubscriptions := []string{"BTC", "ETH"}

		mockDB.On("GetTelegramCryptoSubscriptions", userID).Return(expectedSubscriptions, nil)

		subscriptions, err := mockDB.GetTelegramCryptoSubscriptions(userID)
		assert.NoError(t, err)
		assert.Equal(t, expectedSubscriptions, subscriptions)

		mockDB.AssertExpectations(t)
	})

	t.Run("test delete crypto subscription", func(t *testing.T) {
		userID := 12345
		symbol := "BTC"

		mockDB.On("DeleteTelegramCryptoSubscription", userID, symbol).Return(nil)

		err := mockDB.DeleteTelegramCryptoSubscription(userID, symbol)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})
}

// Benchmark tests
func BenchmarkMockTelegramBot_AddSubscription(b *testing.B) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bot.mu.Lock()
		bot.subscriptions[12345] = append(bot.subscriptions[12345], "USD")
		bot.mu.Unlock()
	}
}

func BenchmarkMockTelegramBot_ReadSubscriptions(b *testing.B) {
	mockDB := &MockDatabase{}
	bot := NewMockTelegramBot(mockDB)

	// Setup some data
	bot.subscriptions[12345] = []string{"USD", "EUR", "GBP"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bot.mu.RLock()
		_ = bot.subscriptions[12345]
		bot.mu.RUnlock()
	}
}
