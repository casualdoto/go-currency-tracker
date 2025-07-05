package scheduler

import (
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
)

// TelegramScheduler schedules Telegram bot tasks
type TelegramScheduler struct {
	bot       *alert.TelegramBot
	ticker    *time.Ticker
	done      chan bool
	isRunning bool
}

// NewTelegramScheduler creates a new TelegramScheduler
func NewTelegramScheduler(bot *alert.TelegramBot) *TelegramScheduler {
	return &TelegramScheduler{
		bot:  bot,
		done: make(chan bool),
	}
}

// RunNow sends daily update immediately (for testing)
func (s *TelegramScheduler) RunNow() {
	log.Println("Running Telegram update immediately for testing")
	s.bot.SendDailyUpdates()
}

// StartDailyUpdates starts sending daily updates at the specified hour
func (s *TelegramScheduler) StartDailyUpdates(hour int) {
	if s.isRunning {
		log.Println("Telegram scheduler is already running")
		return
	}

	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if now.After(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	initialDelay := nextRun.Sub(now)
	log.Printf("Scheduling first Telegram update in %v", initialDelay)

	time.AfterFunc(initialDelay, func() {
		log.Println("Sending initial daily Telegram update")
		s.bot.SendDailyUpdates()

		s.ticker = time.NewTicker(24 * time.Hour)
		s.isRunning = true

		go func() {
			for {
				select {
				case <-s.ticker.C:
					log.Println("Sending scheduled daily Telegram update")
					s.bot.SendDailyUpdates()
				case <-s.done:
					s.ticker.Stop()
					s.ticker = nil
					s.isRunning = false
					log.Println("Telegram scheduler stopped")
					return
				}
			}
		}()
	})
}

// Stop stops the Telegram scheduler
func (s *TelegramScheduler) Stop() {
	if !s.isRunning {
		log.Println("Telegram scheduler is not running")
		return
	}
	s.done <- true
}
