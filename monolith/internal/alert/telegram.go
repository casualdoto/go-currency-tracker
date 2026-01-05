package alert

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency/binance"
	currency "github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
	"github.com/tucnak/telebot"
)

// Subscription represents a user's currency subscription
type Subscription struct {
	UserID    int
	Currency  string
	CreatedAt time.Time
}

// TelegramBot represents a Telegram bot instance
type TelegramBot struct {
	bot              *telebot.Bot
	subscriptions    map[int][]string   // UserID -> []Currency (in-memory cache)
	cryptoSubs       map[int][]string   // UserID -> []CryptoSymbol (in-memory cache)
	lastCryptoPrices map[string]float64 // Symbol -> Last price for change calculation
	mu               sync.RWMutex
	db               *storage.PostgresDB
}

// NewTelegramBot creates a new Telegram bot instance
func NewTelegramBot(token string, db *storage.PostgresDB) (*TelegramBot, error) {
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		return nil, err
	}

	// Initialize the subscriptions table if needed
	if err := db.UpdateSchema(); err != nil {
		return nil, fmt.Errorf("failed to update database schema: %w", err)
	}

	// Load subscriptions from database
	subscriptions, err := db.GetAllTelegramSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("failed to load subscriptions from database: %w", err)
	}

	// Load crypto subscriptions from database
	cryptoSubs, err := db.GetAllTelegramCryptoSubscriptions()
	if err != nil {
		log.Printf("Failed to load crypto subscriptions from database: %v", err)
		cryptoSubs = make(map[int][]string)
	}

	return &TelegramBot{
		bot:              bot,
		subscriptions:    subscriptions,
		cryptoSubs:       cryptoSubs,
		lastCryptoPrices: make(map[string]float64),
		mu:               sync.RWMutex{},
		db:               db,
	}, nil
}

// Start starts the bot
func (t *TelegramBot) Start() {
	// Handle /start command
	t.bot.Handle("/start", func(m *telebot.Message) {
		msg := "Welcome to Currency Tracker Bot!\n\n" +
			"Available commands:\n" +
			"/currencies - Get list of available currencies\n" +
			"/subscribe [currency] - Subscribe to currency updates (e.g., /subscribe USD)\n" +
			"/unsubscribe [currency] - Unsubscribe from currency updates (e.g., /unsubscribe USD)\n" +
			"/list - List your subscriptions\n" +
			"/rate [currency] - Get current rate for a currency (e.g., /rate USD)\n\n" +
			"Cryptocurrency commands:\n" +
			"/cryptocurrencies - Get list of available cryptocurrencies\n" +
			"/crypto_subscribe [symbol] - Subscribe to crypto updates (e.g., /crypto_subscribe BTC)\n" +
			"/crypto_unsubscribe [symbol] - Unsubscribe from crypto updates (e.g., /crypto_unsubscribe BTC)\n" +
			"/crypto_list - List your crypto subscriptions\n" +
			"/crypto_rate [symbol] - Get current rate for a cryptocurrency (e.g., /crypto_rate BTC)"

		t.bot.Send(m.Sender, msg)
	})

	// Handle /currencies command
	t.bot.Handle("/currencies", func(m *telebot.Message) {
		// Get available currencies from CBR
		rates, err := currency.GetCBRRatesByDate("")
		if err != nil {
			t.bot.Send(m.Sender, "Failed to retrieve available currencies. Please try again later.")
			return
		}

		msg := "📋 Available Currencies:\n\n"

		// Sort currencies by code
		type currencyInfo struct {
			code string
			name string
		}

		var currencies []currencyInfo
		for code, rate := range rates.Valute {
			currencies = append(currencies, currencyInfo{code: code, name: rate.Name})
		}

		// Simple sorting by code
		for i := 0; i < len(currencies)-1; i++ {
			for j := i + 1; j < len(currencies); j++ {
				if currencies[i].code > currencies[j].code {
					currencies[i], currencies[j] = currencies[j], currencies[i]
				}
			}
		}

		for _, curr := range currencies {
			msg += fmt.Sprintf("💱 %s - %s\n", curr.code, curr.name)
		}

		msg += "\nUse /subscribe [currency] to subscribe to updates"

		t.bot.Send(m.Sender, msg)
	})

	// Handle /cryptocurrencies command
	t.bot.Handle("/cryptocurrencies", func(m *telebot.Message) {
		msg := "🪙 Available Cryptocurrencies:\n\n"

		// Fixed list of popular cryptocurrencies
		cryptocurrencies := []struct {
			symbol string
			name   string
		}{
			{"BTC", "Bitcoin"},
			{"ETH", "Ethereum"},
			{"BNB", "Binance Coin"},
			{"SOL", "Solana"},
			{"XRP", "XRP"},
			{"ADA", "Cardano"},
			{"AVAX", "Avalanche"},
			{"DOT", "Polkadot"},
			{"DOGE", "Dogecoin"},
			{"SHIB", "Shiba Inu"},
			{"LINK", "Chainlink"},
			{"MATIC", "Polygon"},
			{"UNI", "Uniswap"},
			{"LTC", "Litecoin"},
			{"ATOM", "Cosmos"},
			{"XTZ", "Tezos"},
			{"FIL", "Filecoin"},
			{"TRX", "TRON"},
			{"ETC", "Ethereum Classic"},
			{"NEAR", "NEAR Protocol"},
		}

		for _, crypto := range cryptocurrencies {
			msg += fmt.Sprintf("₿ %s - %s\n", crypto.symbol, crypto.name)
		}

		msg += "\nUse /crypto_subscribe [symbol] to subscribe to updates"

		t.bot.Send(m.Sender, msg)
	})

	// Handle /subscribe command
	t.bot.Handle("/subscribe", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a currency code. Example: /subscribe USD")
			return
		}

		currencyCode := strings.ToUpper(args[1])

		// Verify the currency exists
		_, err := currency.GetCurrencyRate(currencyCode, "")
		if err != nil {
			t.bot.Send(m.Sender, fmt.Sprintf("Currency %s not found or unavailable", currencyCode))
			return
		}

		t.mu.Lock()
		defer t.mu.Unlock()

		// Check if user already has subscriptions
		currencies, exists := t.subscriptions[m.Sender.ID]
		if !exists {
			t.subscriptions[m.Sender.ID] = []string{currencyCode}
		} else {
			// Check if already subscribed
			for _, c := range currencies {
				if c == currencyCode {
					t.bot.Send(m.Sender, fmt.Sprintf("You are already subscribed to %s", currencyCode))
					return
				}
			}
			t.subscriptions[m.Sender.ID] = append(currencies, currencyCode)
		}

		// Save to database
		err = t.db.SaveTelegramSubscription(m.Sender.ID, currencyCode)
		if err != nil {
			log.Printf("Error saving subscription to database: %v", err)
			t.bot.Send(m.Sender, "Failed to save subscription. Please try again later.")
			return
		}

		t.bot.Send(m.Sender, fmt.Sprintf("You have successfully subscribed to %s", currencyCode))
	})

	// Handle /unsubscribe command
	t.bot.Handle("/unsubscribe", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a currency code. Example: /unsubscribe USD")
			return
		}

		currencyCode := strings.ToUpper(args[1])

		t.mu.Lock()
		defer t.mu.Unlock()

		currencies, exists := t.subscriptions[m.Sender.ID]
		if !exists {
			t.bot.Send(m.Sender, "You don't have any subscriptions")
			return
		}

		found := false
		newCurrencies := []string{}
		for _, c := range currencies {
			if c != currencyCode {
				newCurrencies = append(newCurrencies, c)
			} else {
				found = true
			}
		}

		if !found {
			t.bot.Send(m.Sender, fmt.Sprintf("You are not subscribed to %s", currencyCode))
			return
		}

		// Delete from database
		err := t.db.DeleteTelegramSubscription(m.Sender.ID, currencyCode)
		if err != nil {
			log.Printf("Error deleting subscription from database: %v", err)
			t.bot.Send(m.Sender, "Failed to unsubscribe. Please try again later.")
			return
		}

		t.subscriptions[m.Sender.ID] = newCurrencies
		t.bot.Send(m.Sender, fmt.Sprintf("You have successfully unsubscribed from %s", currencyCode))
	})

	// Handle /list command
	t.bot.Handle("/list", func(m *telebot.Message) {
		// Get subscriptions from database to ensure we have the latest data
		currencies, err := t.db.GetTelegramSubscriptions(m.Sender.ID)
		if err != nil {
			log.Printf("Error getting subscriptions from database: %v", err)
			t.bot.Send(m.Sender, "Failed to retrieve your subscriptions. Please try again later.")
			return
		}

		if len(currencies) == 0 {
			t.bot.Send(m.Sender, "You don't have any subscriptions")
			return
		}

		msg := "Your subscriptions:\n"
		for _, c := range currencies {
			msg += "- " + c + "\n"
		}

		t.bot.Send(m.Sender, msg)

		// Update in-memory cache
		t.mu.Lock()
		t.subscriptions[m.Sender.ID] = currencies
		t.mu.Unlock()
	})

	// Handle /rate command
	t.bot.Handle("/rate", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a currency code. Example: /rate USD")
			return
		}

		currencyCode := strings.ToUpper(args[1])

		// Get current rate
		rate, err := currency.GetCurrencyRate(currencyCode, "")
		if err != nil {
			t.bot.Send(m.Sender, fmt.Sprintf("Error getting rate for %s: %v", currencyCode, err))
			return
		}

		// Format the message
		msg := fmt.Sprintf("Currency: %s (%s)\n", rate.Name, rate.CharCode)
		msg += fmt.Sprintf("Current rate: %.4f RUB (per %d unit)\n", rate.Value, rate.Nominal)
		msg += fmt.Sprintf("Previous rate: %.4f RUB", rate.Previous)

		t.bot.Send(m.Sender, msg)
	})

	// Handle /crypto_subscribe command
	t.bot.Handle("/crypto_subscribe", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a cryptocurrency symbol. Example: /crypto_subscribe BTC")
			return
		}

		symbol := strings.ToUpper(args[1])

		// Verify the cryptocurrency exists by trying to get its current rate
		client := binance.NewClient()
		_, err := client.GetCurrentCryptoToRubRate(symbol)
		if err != nil {
			t.bot.Send(m.Sender, fmt.Sprintf("Cryptocurrency %s not found or unavailable: %v", symbol, err))
			return
		}

		t.mu.Lock()
		defer t.mu.Unlock()

		// Check if user already has crypto subscriptions
		symbols, exists := t.cryptoSubs[m.Sender.ID]
		if !exists {
			t.cryptoSubs[m.Sender.ID] = []string{symbol}
		} else {
			// Check if already subscribed
			for _, s := range symbols {
				if s == symbol {
					t.bot.Send(m.Sender, fmt.Sprintf("You are already subscribed to %s", symbol))
					return
				}
			}
			t.cryptoSubs[m.Sender.ID] = append(symbols, symbol)
		}

		// Save to database
		err = t.db.SaveTelegramCryptoSubscription(m.Sender.ID, symbol)
		if err != nil {
			log.Printf("Error saving crypto subscription to database: %v", err)
			t.bot.Send(m.Sender, "Failed to save subscription. Please try again later.")
			return
		}

		t.bot.Send(m.Sender, fmt.Sprintf("You have successfully subscribed to %s", symbol))
	})

	// Handle /crypto_unsubscribe command
	t.bot.Handle("/crypto_unsubscribe", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a cryptocurrency symbol. Example: /crypto_unsubscribe BTC")
			return
		}

		symbol := strings.ToUpper(args[1])

		t.mu.Lock()
		defer t.mu.Unlock()

		symbols, exists := t.cryptoSubs[m.Sender.ID]
		if !exists {
			t.bot.Send(m.Sender, "You don't have any cryptocurrency subscriptions")
			return
		}

		found := false
		newSymbols := []string{}
		for _, s := range symbols {
			if s != symbol {
				newSymbols = append(newSymbols, s)
			} else {
				found = true
			}
		}

		if !found {
			t.bot.Send(m.Sender, fmt.Sprintf("You are not subscribed to %s", symbol))
			return
		}

		// Delete from database
		err := t.db.DeleteTelegramCryptoSubscription(m.Sender.ID, symbol)
		if err != nil {
			log.Printf("Error deleting crypto subscription from database: %v", err)
			t.bot.Send(m.Sender, "Failed to unsubscribe. Please try again later.")
			return
		}

		t.cryptoSubs[m.Sender.ID] = newSymbols
		t.bot.Send(m.Sender, fmt.Sprintf("You have successfully unsubscribed from %s", symbol))
	})

	// Handle /crypto_list command
	t.bot.Handle("/crypto_list", func(m *telebot.Message) {
		// Get crypto subscriptions from database to ensure we have the latest data
		symbols, err := t.db.GetTelegramCryptoSubscriptions(m.Sender.ID)
		if err != nil {
			log.Printf("Error getting crypto subscriptions from database: %v", err)
			t.bot.Send(m.Sender, "Failed to retrieve your cryptocurrency subscriptions. Please try again later.")
			return
		}

		if len(symbols) == 0 {
			t.bot.Send(m.Sender, "You don't have any cryptocurrency subscriptions")
			return
		}

		msg := "Your cryptocurrency subscriptions:\n"
		for _, s := range symbols {
			msg += "- " + s + "\n"
		}

		t.bot.Send(m.Sender, msg)

		// Update in-memory cache
		t.mu.Lock()
		t.cryptoSubs[m.Sender.ID] = symbols
		t.mu.Unlock()
	})

	// Handle /crypto_rate command
	t.bot.Handle("/crypto_rate", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) < 2 {
			t.bot.Send(m.Sender, "Please specify a cryptocurrency symbol. Example: /crypto_rate BTC")
			return
		}

		symbol := strings.ToUpper(args[1])

		// Get current rate
		client := binance.NewClient()
		rate, err := client.GetCurrentCryptoToRubRate(symbol)
		if err != nil {
			t.bot.Send(m.Sender, fmt.Sprintf("Error getting rate for %s: %v", symbol, err))
			return
		}

		// Get crypto name
		cryptoNames := map[string]string{
			"BTC":   "Bitcoin",
			"ETH":   "Ethereum",
			"BNB":   "Binance Coin",
			"SOL":   "Solana",
			"ADA":   "Cardano",
			"XRP":   "Ripple",
			"DOT":   "Polkadot",
			"DOGE":  "Dogecoin",
			"SHIB":  "Shiba Inu",
			"MATIC": "Polygon",
			"AVAX":  "Avalanche",
		}
		cryptoName := cryptoNames[symbol]
		if cryptoName == "" {
			cryptoName = symbol
		}

		// Format the message
		msg := fmt.Sprintf("Cryptocurrency: %s (%s)\n", cryptoName, symbol)
		msg += fmt.Sprintf("Current rate: %.2f RUB\n", rate.Close)
		msg += fmt.Sprintf("24h High: %.2f RUB\n", rate.High)
		msg += fmt.Sprintf("24h Low: %.2f RUB", rate.Low)

		t.bot.Send(m.Sender, msg)
	})

	// Start the bot
	go t.bot.Start()
}

// SendDailyUpdates sends daily updates to all subscribers
func (t *TelegramBot) SendDailyUpdates() {
	// Refresh subscriptions from database
	subscriptions, err := t.db.GetAllTelegramSubscriptions()
	if err != nil {
		log.Printf("Error refreshing subscriptions from database: %v", err)
	} else {
		t.mu.Lock()
		t.subscriptions = subscriptions
		t.mu.Unlock()
	}

	// Refresh crypto subscriptions from database
	cryptoSubs, err := t.db.GetAllTelegramCryptoSubscriptions()
	if err != nil {
		log.Printf("Error refreshing crypto subscriptions from database: %v", err)
	} else {
		t.mu.Lock()
		t.cryptoSubs = cryptoSubs
		t.mu.Unlock()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Send currency updates
	for userID, currencies := range t.subscriptions {
		if len(currencies) == 0 {
			continue
		}

		// Create a message for this user
		msg := "📊 Daily Currency Update 📊\n\n"

		for _, code := range currencies {
			// Get current rate
			rate, err := currency.GetCurrencyRate(code, "")
			if err != nil {
				log.Printf("Error getting rate for %s: %v", code, err)
				continue
			}

			// Calculate change percentage
			changePercent := ((rate.Value - rate.Previous) / rate.Previous) * 100
			changeEmoji := "🔄"
			if changePercent > 0 {
				changeEmoji = "📈"
			} else if changePercent < 0 {
				changeEmoji = "📉"
			}

			// Add to message
			msg += fmt.Sprintf("%s %s (%s): %.4f RUB (%.2f%%)\n",
				changeEmoji, rate.Name, rate.CharCode, rate.Value, changePercent)
		}

		// Send message
		user := &telebot.User{ID: userID}
		_, err := t.bot.Send(user, msg)
		if err != nil {
			log.Printf("Error sending message to user %d: %v", userID, err)
		}
	}

	// Send cryptocurrency updates
	for userID, symbols := range t.cryptoSubs {
		if len(symbols) == 0 {
			continue
		}

		// Create a message for this user
		msg := "💰 Daily Cryptocurrency Update 💰\n\n"

		client := binance.NewClient()

		for _, symbol := range symbols {
			// Get current rate
			rate, err := client.GetCurrentCryptoToRubRate(symbol)
			if err != nil {
				log.Printf("Error getting crypto rate for %s: %v", symbol, err)
				continue
			}

			log.Printf("SendDailyUpdates: Got rate for %s: Close=%.2f RUB", symbol, rate.Close)

			// Get crypto name
			cryptoNames := map[string]string{
				"BTC":   "Bitcoin",
				"ETH":   "Ethereum",
				"BNB":   "Binance Coin",
				"SOL":   "Solana",
				"ADA":   "Cardano",
				"XRP":   "Ripple",
				"DOT":   "Polkadot",
				"DOGE":  "Dogecoin",
				"SHIB":  "Shiba Inu",
				"MATIC": "Polygon",
				"AVAX":  "Avalanche",
			}
			cryptoName := cryptoNames[symbol]
			if cryptoName == "" {
				cryptoName = symbol
			}

			// Add to message
			msg += fmt.Sprintf("💲 %s (%s): %.2f RUB\n",
				cryptoName, symbol, rate.Close)
		}

		// Send message
		user := &telebot.User{ID: userID}
		_, err := t.bot.Send(user, msg)
		if err != nil {
			log.Printf("Error sending crypto message to user %d: %v", userID, err)
		}
	}
}

// SendCryptoUpdates sends 15-minute crypto updates to all subscribers
func (t *TelegramBot) SendCryptoUpdates() {
	// Refresh crypto subscriptions from database
	cryptoSubs, err := t.db.GetAllTelegramCryptoSubscriptions()
	if err != nil {
		log.Printf("Error refreshing crypto subscriptions from database: %v", err)
		// Continue with in-memory cache if available
	} else {
		t.mu.Lock()
		t.cryptoSubs = cryptoSubs
		t.mu.Unlock()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Collect all unique symbols to fetch
	allSymbols := make(map[string]bool)
	for _, symbols := range t.cryptoSubs {
		for _, symbol := range symbols {
			allSymbols[symbol] = true
		}
	}

	if len(allSymbols) == 0 {
		return
	}

	// Get current rates for all symbols
	client := binance.NewClient()
	currentPrices := make(map[string]*binance.CryptoRate)

	for symbol := range allSymbols {
		rate, err := client.GetCurrentCryptoToRubRate(symbol)
		if err != nil {
			log.Printf("Error getting crypto rate for %s: %v", symbol, err)
			continue
		}
		currentPrices[symbol] = rate
	}

	// Send updates to each user
	for userID, symbols := range t.cryptoSubs {
		if len(symbols) == 0 {
			continue
		}

		hasSignificantChange := false
		msg := "📊 Crypto Update 📊\n\n"

		// Get crypto names map
		cryptoNames := map[string]string{
			"BTC":   "Bitcoin",
			"ETH":   "Ethereum",
			"BNB":   "Binance Coin",
			"SOL":   "Solana",
			"ADA":   "Cardano",
			"XRP":   "XRP",
			"DOT":   "Polkadot",
			"DOGE":  "Dogecoin",
			"SHIB":  "Shiba Inu",
			"MATIC": "Polygon",
			"AVAX":  "Avalanche",
			"LINK":  "Chainlink",
			"UNI":   "Uniswap",
			"LTC":   "Litecoin",
			"ATOM":  "Cosmos",
			"XTZ":   "Tezos",
			"FIL":   "Filecoin",
			"TRX":   "TRON",
			"ETC":   "Ethereum Classic",
			"NEAR":  "NEAR Protocol",
		}

		for _, symbol := range symbols {
			rate, exists := currentPrices[symbol]
			if !exists {
				continue
			}

			cryptoName := cryptoNames[symbol]
			if cryptoName == "" {
				cryptoName = symbol
			}

			currentPrice := rate.Close
			lastPrice, hadPrevious := t.lastCryptoPrices[symbol]

			if hadPrevious {
				// Calculate change percentage
				changePercent := ((currentPrice - lastPrice) / lastPrice) * 100
				changeEmoji := "🔄"

				// Only show notification if change is significant (>= 2%)
				if changePercent >= 2.0 {
					changeEmoji = "📈"
					hasSignificantChange = true
				} else if changePercent <= -2.0 {
					changeEmoji = "📉"
					hasSignificantChange = true
				} else if changePercent == 0 {
					changeEmoji = "🔄"
				} else {
					// Small change, still show but mark as insignificant
					if changePercent > 0 {
						changeEmoji = "📈"
					} else {
						changeEmoji = "📉"
					}
				}

				// Add to message
				msg += fmt.Sprintf("%s %s (%s): %.2f ₽ (%+.2f%%)\n",
					changeEmoji, cryptoName, symbol, currentPrice, changePercent)
			} else {
				// First time seeing this crypto
				msg += fmt.Sprintf("💲 %s (%s): %.2f ₽ (new)\n",
					cryptoName, symbol, currentPrice)
				hasSignificantChange = true
			}

			// Update last price
			t.lastCryptoPrices[symbol] = currentPrice
		}

		// Only send message if there are significant changes (>= 2%) or it's first time
		if hasSignificantChange {
			user := &telebot.User{ID: userID}
			_, err := t.bot.Send(user, msg)
			if err != nil {
				log.Printf("Error sending crypto update to user %d: %v", userID, err)
			}
		}
	}
}

// Stop stops the bot
func (t *TelegramBot) Stop() {
	t.bot.Stop()
}
