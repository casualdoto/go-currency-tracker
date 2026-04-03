package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/telegram-bot/internal/config"
	"github.com/tucnak/telebot"
)

// Bot wraps the Telegram bot and calls upstream services.
type Bot struct {
	bot        *telebot.Bot
	cfg        *config.Config
	httpClient *http.Client
}

func New(cfg *config.Config) (*Bot, error) {
	b, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}
	return &Bot{
		bot:        b,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (b *Bot) Start() {
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/rates", b.handleRates)
	b.bot.Handle("/subscribe", b.handleSubscribe)
	b.bot.Handle("/unsubscribe", b.handleUnsubscribe)
	b.bot.Handle("/history", b.handleHistory)
	b.bot.Handle("/crypto_subscribe", b.handleCryptoSubscribe)
	b.bot.Handle("/crypto_unsubscribe", b.handleCryptoUnsubscribe)

	// If a webhook was set (e.g. from another deploy), getUpdates receives nothing.
	if _, err := b.bot.Raw("deleteWebhook", map[string]interface{}{}); err != nil {
		log.Printf("telegram: deleteWebhook: %v", err)
	} else {
		log.Printf("telegram: webhook cleared, long polling enabled")
	}
	if b.bot.Me != nil {
		log.Printf("telegram: bot @%s ready", b.bot.Me.Username)
	}

	go b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) handleStart(m *telebot.Message) {
	msg := "Welcome to Currency Tracker Bot!\n\n" +
		"Commands:\n" +
		"/rates - Get current CBR rates\n" +
		"/subscribe [CURRENCY] - Subscribe to daily updates (e.g. /subscribe USD)\n" +
		"/unsubscribe [CURRENCY] - Unsubscribe\n" +
		"/history [CURRENCY] - Get 7-day history (e.g. /history USD)\n" +
		"/crypto_subscribe [SYMBOL] - Subscribe to crypto (e.g. /crypto_subscribe BTC)\n" +
		"/crypto_unsubscribe [SYMBOL] - Unsubscribe from crypto"
	b.bot.Send(m.Sender, msg)
}

func (b *Bot) handleRates(m *telebot.Message) {
	url := fmt.Sprintf("%s/rates/cbr", b.cfg.APIGatewayURL)
	resp, err := b.httpClient.Get(url)
	if err != nil {
		b.bot.Send(m.Sender, "Failed to fetch rates. Please try again later.")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rates []struct {
		CurrencyCode string  `json:"CurrencyCode"`
		CurrencyName string  `json:"CurrencyName"`
		Value        float64 `json:"Value"`
		Previous     float64 `json:"Previous"`
	}
	if err := json.Unmarshal(body, &rates); err != nil || len(rates) == 0 {
		b.bot.Send(m.Sender, "No rate data available right now.")
		return
	}

	msg := "📊 Current CBR Rates:\n\n"
	for _, r := range rates {
		change := ((r.Value - r.Previous) / r.Previous) * 100
		emoji := "🔄"
		if change > 0 {
			emoji = "📈"
		} else if change < 0 {
			emoji = "📉"
		}
		msg += fmt.Sprintf("%s %s (%s): %.4f RUB (%+.2f%%)\n", emoji, r.CurrencyName, r.CurrencyCode, r.Value, change)
	}
	b.bot.Send(m.Sender, msg)
}

func (b *Bot) handleSubscribe(m *telebot.Message) {
	args := strings.Fields(m.Text)
	if len(args) < 2 {
		b.bot.Send(m.Sender, "Usage: /subscribe USD")
		return
	}
	currency := strings.ToUpper(args[1])
	if err := b.subscribeCBR(m.Sender.ID, currency); err != nil {
		b.bot.Send(m.Sender, fmt.Sprintf("Failed to subscribe: %v", err))
		return
	}
	b.bot.Send(m.Sender, fmt.Sprintf("Subscribed to %s updates!", currency))
}

func (b *Bot) handleUnsubscribe(m *telebot.Message) {
	args := strings.Fields(m.Text)
	if len(args) < 2 {
		b.bot.Send(m.Sender, "Usage: /unsubscribe USD")
		return
	}
	currency := strings.ToUpper(args[1])
	if err := b.unsubscribeCBR(m.Sender.ID, currency); err != nil {
		b.bot.Send(m.Sender, fmt.Sprintf("Failed to unsubscribe: %v", err))
		return
	}
	b.bot.Send(m.Sender, fmt.Sprintf("Unsubscribed from %s.", currency))
}

func (b *Bot) handleCryptoSubscribe(m *telebot.Message) {
	args := strings.Fields(m.Text)
	if len(args) < 2 {
		b.bot.Send(m.Sender, "Usage: /crypto_subscribe BTC")
		return
	}
	symbol := strings.ToUpper(args[1])
	if err := b.subscribeCrypto(m.Sender.ID, symbol); err != nil {
		b.bot.Send(m.Sender, fmt.Sprintf("Failed to subscribe: %v", err))
		return
	}
	b.bot.Send(m.Sender, fmt.Sprintf("Subscribed to %s crypto updates!", symbol))
}

func (b *Bot) handleCryptoUnsubscribe(m *telebot.Message) {
	args := strings.Fields(m.Text)
	if len(args) < 2 {
		b.bot.Send(m.Sender, "Usage: /crypto_unsubscribe BTC")
		return
	}
	symbol := strings.ToUpper(args[1])
	if err := b.unsubscribeCrypto(m.Sender.ID, symbol); err != nil {
		b.bot.Send(m.Sender, fmt.Sprintf("Failed to unsubscribe: %v", err))
		return
	}
	b.bot.Send(m.Sender, fmt.Sprintf("Unsubscribed from %s.", symbol))
}

func (b *Bot) handleHistory(m *telebot.Message) {
	args := strings.Fields(m.Text)
	if len(args) < 2 {
		b.bot.Send(m.Sender, "Usage: /history USD")
		return
	}
	currency := strings.ToUpper(args[1])
	to := time.Now()
	from := to.AddDate(0, 0, -7)
	url := fmt.Sprintf("%s/rates/cbr/range?code=%s&from=%s&to=%s",
		b.cfg.APIGatewayURL, currency,
		from.Format("2006-01-02"), to.Format("2006-01-02"))

	resp, err := b.httpClient.Get(url)
	if err != nil || resp.StatusCode != 200 {
		b.bot.Send(m.Sender, "Failed to fetch history.")
		return
	}
	defer resp.Body.Close()

	var rates []struct {
		Date  string  `json:"Date"`
		Value float64 `json:"Value"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &rates); err != nil || len(rates) == 0 {
		b.bot.Send(m.Sender, "No history data available.")
		return
	}

	msg := fmt.Sprintf("📈 %s history (7 days):\n\n", currency)
	for _, r := range rates {
		msg += fmt.Sprintf("%s: %.4f RUB\n", r.Date, r.Value)
	}
	b.bot.Send(m.Sender, msg)
}

func (b *Bot) subscribeCBR(telegramID int, currency string) error {
	return b.postToNotification("/subscriptions/cbr", map[string]any{
		"telegram_id": int64(telegramID),
		"value":       currency,
	})
}

func (b *Bot) unsubscribeCBR(telegramID int, currency string) error {
	return b.deleteFromNotification("/subscriptions/cbr", map[string]any{
		"telegram_id": int64(telegramID),
		"value":       currency,
	})
}

func (b *Bot) subscribeCrypto(telegramID int, symbol string) error {
	return b.postToNotification("/subscriptions/crypto", map[string]any{
		"telegram_id": int64(telegramID),
		"value":       symbol,
	})
}

func (b *Bot) unsubscribeCrypto(telegramID int, symbol string) error {
	return b.deleteFromNotification("/subscriptions/crypto", map[string]any{
		"telegram_id": int64(telegramID),
		"value":       symbol,
	})
}

func (b *Bot) postToNotification(path string, body any) error {
	data, _ := json.Marshal(body)
	url := b.cfg.NotificationSvcURL + path
	resp, err := b.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification service returned %d", resp.StatusCode)
	}
	return nil
}

func (b *Bot) deleteFromNotification(path string, body any) error {
	data, _ := json.Marshal(body)
	url := b.cfg.NotificationSvcURL + path
	req, _ := http.NewRequest(http.MethodDelete, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification service returned %d", resp.StatusCode)
	}
	return nil
}
