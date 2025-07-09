package scheduler

import (
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/alert"
)

// TelegramScheduler schedules Telegram bot tasks
type TelegramScheduler struct {
	bot             *alert.TelegramBot
	dailyTicker     *time.Ticker
	cryptoTicker    *time.Ticker
	dailyDone       chan bool
	cryptoDone      chan bool
	isDailyRunning  bool
	isCryptoRunning bool
}

// NewTelegramScheduler creates a new TelegramScheduler
func NewTelegramScheduler(bot *alert.TelegramBot) *TelegramScheduler {
	return &TelegramScheduler{
		bot:        bot,
		dailyDone:  make(chan bool),
		cryptoDone: make(chan bool),
	}
}

// RunNow sends daily update immediately (for testing)
func (s *TelegramScheduler) RunNow() {
	log.Println("Running Telegram update immediately for testing")
	s.bot.SendDailyUpdates()
}

// RunCryptoNow sends crypto update immediately (for testing)
func (s *TelegramScheduler) RunCryptoNow() {
	log.Println("Running crypto Telegram update immediately for testing")
	s.bot.SendCryptoUpdates()
}

// StartDailyUpdates starts sending daily updates at the specified hour
func (s *TelegramScheduler) StartDailyUpdates(hour int) {
	if s.isDailyRunning {
		log.Println("Daily Telegram scheduler is already running")
		return
	}

	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if now.After(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	initialDelay := nextRun.Sub(now)
	log.Printf("Scheduling first daily Telegram update in %v", initialDelay)

	time.AfterFunc(initialDelay, func() {
		log.Println("Sending initial daily Telegram update")
		s.bot.SendDailyUpdates()

		s.dailyTicker = time.NewTicker(24 * time.Hour)
		s.isDailyRunning = true

		go func() {
			for {
				select {
				case <-s.dailyTicker.C:
					log.Println("Sending scheduled daily Telegram update")
					s.bot.SendDailyUpdates()
				case <-s.dailyDone:
					s.dailyTicker.Stop()
					s.dailyTicker = nil
					s.isDailyRunning = false
					log.Println("Daily Telegram scheduler stopped")
					return
				}
			}
		}()
	})
}

// StartCryptoUpdates starts sending crypto updates every 15 minutes
func (s *TelegramScheduler) StartCryptoUpdates() {
	if s.isCryptoRunning {
		log.Println("Crypto Telegram scheduler is already running")
		return
	}

	log.Println("Starting crypto updates every 15 minutes")

	// Send initial update
	s.bot.SendCryptoUpdates()

	s.cryptoTicker = time.NewTicker(15 * time.Minute)
	s.isCryptoRunning = true

	go func() {
		for {
			select {
			case <-s.cryptoTicker.C:
				log.Println("Sending scheduled crypto Telegram update")
				s.bot.SendCryptoUpdates()
			case <-s.cryptoDone:
				s.cryptoTicker.Stop()
				s.cryptoTicker = nil
				s.isCryptoRunning = false
				log.Println("Crypto Telegram scheduler stopped")
				return
			}
		}
	}()
}

// StopDaily stops the daily Telegram scheduler
func (s *TelegramScheduler) StopDaily() {
	if !s.isDailyRunning {
		log.Println("Daily Telegram scheduler is not running")
		return
	}
	s.dailyDone <- true
}

// StopCrypto stops the crypto Telegram scheduler
func (s *TelegramScheduler) StopCrypto() {
	if !s.isCryptoRunning {
		log.Println("Crypto Telegram scheduler is not running")
		return
	}
	s.cryptoDone <- true
}

// Stop stops both schedulers
func (s *TelegramScheduler) Stop() {
	s.StopDaily()
	s.StopCrypto()
}
