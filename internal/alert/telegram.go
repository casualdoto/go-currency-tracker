package alert

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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
	bot           *telebot.Bot
	subscriptions map[int][]string // UserID -> []Currency (in-memory cache)
	mu            sync.RWMutex
	db            *storage.PostgresDB
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

	return &TelegramBot{
		bot:           bot,
		subscriptions: subscriptions,
		mu:            sync.RWMutex{},
		db:            db,
	}, nil
}

// Start starts the bot
func (t *TelegramBot) Start() {
	// Handle /start command
	t.bot.Handle("/start", func(m *telebot.Message) {
		msg := "Welcome to Currency Tracker Bot!\n\n" +
			"Available commands:\n" +
			"/subscribe [currency] - Subscribe to currency updates (e.g., /subscribe USD)\n" +
			"/unsubscribe [currency] - Unsubscribe from currency updates (e.g., /unsubscribe USD)\n" +
			"/list - List your subscriptions\n" +
			"/rate [currency] - Get current rate for a currency (e.g., /rate USD)"

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

	// Start the bot
	go t.bot.Start()
}

// SendDailyUpdates sends daily updates to all subscribers
func (t *TelegramBot) SendDailyUpdates() {
	// Refresh subscriptions from database
	subscriptions, err := t.db.GetAllTelegramSubscriptions()
	if err != nil {
		log.Printf("Error refreshing subscriptions from database: %v", err)
		// Continue with in-memory cache if available
	} else {
		t.mu.Lock()
		t.subscriptions = subscriptions
		t.mu.Unlock()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

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
				log.Printf("Error getting current rate for %s: %v", code, err)
				continue
			}

			// Format the message
			msg += fmt.Sprintf("Currency: %s (%s)\n", rate.Name, rate.CharCode)
			msg += fmt.Sprintf("Current rate: %.4f RUB (per %d unit)\n", rate.Value, rate.Nominal)
			msg += fmt.Sprintf("Previous rate: %.4f RUB\n", rate.Previous)

			// Calculate change percentage
			changePercent := ((rate.Value - rate.Previous) / rate.Previous) * 100
			if changePercent > 0 {
				msg += fmt.Sprintf("Change: +%.2f%%\n\n", changePercent)
			} else {
				msg += fmt.Sprintf("Change: %.2f%%\n\n", changePercent)
			}
		}

		// Send the message
		user := &telebot.User{ID: userID}
		_, err := t.bot.Send(user, msg)
		if err != nil {
			log.Printf("Error sending daily update to user %d: %v", userID, err)
		}
	}
}

// Stop stops the bot
func (t *TelegramBot) Stop() {
	t.bot.Stop()
}
