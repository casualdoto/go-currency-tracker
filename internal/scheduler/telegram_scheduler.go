package scheduler

import (
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
)

// TelegramScheduler schedules Telegram bot tasks
type TelegramScheduler struct {
	bot    *alert.TelegramBot
	ticker *time.Ticker
	done   chan bool
}

// NewTelegramScheduler creates a new TelegramScheduler
func NewTelegramScheduler(bot *alert.TelegramBot) *TelegramScheduler {
	return &TelegramScheduler{
		bot:  bot,
		done: make(chan bool),
	}
}

// StartDailyUpdates starts sending daily updates at the specified hour
func (s *TelegramScheduler) StartDailyUpdates(hour int) {
	// Calculate the time until the next scheduled run
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if now.After(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	// Wait until the next scheduled time
	initialDelay := nextRun.Sub(now)
	log.Printf("Scheduling first daily update in %v", initialDelay)

	time.AfterFunc(initialDelay, func() {
		// Send the first update
		log.Println("Sending daily currency updates")
		s.bot.SendDailyUpdates()

		// Then set up a ticker for every 24 hours
		s.ticker = time.NewTicker(24 * time.Hour)

		go func() {
			for {
				select {
				case <-s.ticker.C:
					log.Println("Sending daily currency updates")
					s.bot.SendDailyUpdates()
				case <-s.done:
					s.ticker.Stop()
					return
				}
			}
		}()
	})
}

// Stop stops the scheduler
func (s *TelegramScheduler) Stop() {
	if s.ticker != nil {
		s.done <- true
	}
}
