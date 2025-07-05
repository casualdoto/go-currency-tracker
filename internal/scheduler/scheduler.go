// Package scheduler provides task scheduling functionality
package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
)

// CurrencyRateScheduler is responsible for scheduling currency rate updates
type CurrencyRateScheduler struct {
	db           *storage.PostgresDB
	stopChan     chan struct{}
	isRunning    bool
	dailyJobTime time.Time
	ticker       *time.Ticker
}

// NewCurrencyRateScheduler creates a new scheduler for currency rate updates
func NewCurrencyRateScheduler(db *storage.PostgresDB, hourUTC, minuteUTC int) *CurrencyRateScheduler {
	now := time.Now().UTC()
	jobTime := time.Date(now.Year(), now.Month(), now.Day(), hourUTC, minuteUTC, 0, 0, time.UTC)

	// Если текущее время уже прошло, сдвигаем на следующий день
	for now.After(jobTime) {
		jobTime = jobTime.Add(24 * time.Hour)
	}

	return &CurrencyRateScheduler{
		db:           db,
		stopChan:     make(chan struct{}),
		isRunning:    false,
		dailyJobTime: jobTime,
	}
}

// Start begins the scheduler
func (s *CurrencyRateScheduler) Start() {
	if s.isRunning {
		log.Println("Currency rate scheduler is already running")
		return
	}

	s.isRunning = true
	log.Printf("Currency rate scheduler started. First update at %s UTC", s.dailyJobTime.Format("2006-01-02 15:04:05"))
	go s.run()
}

// Stop stops the scheduler
func (s *CurrencyRateScheduler) Stop() {
	if !s.isRunning {
		log.Println("Currency rate scheduler is not running")
		return
	}

	s.stopChan <- struct{}{}
	s.isRunning = false
	log.Println("Currency rate scheduler stopping...")
}

// run is the main loop for the scheduler
func (s *CurrencyRateScheduler) run() {
	// calculate delay to first run
	now := time.Now().UTC()
	delay := s.dailyJobTime.Sub(now)
	if delay < 0 {
		delay = 0
	}

	// delayed start
	timer := time.NewTimer(delay)

	select {
	case <-timer.C:
		log.Println("Executing first scheduled currency rate update")
		s.executeJob()
	case <-s.stopChan:
		if !timer.Stop() {
			<-timer.C
		}
		log.Println("Currency rate scheduler stopped before first run")
		return
	}

	s.ticker = time.NewTicker(24 * time.Hour)

	for {
		select {
		case <-s.ticker.C:
			log.Println("Executing daily scheduled currency rate update")
			s.executeJob()
		case <-s.stopChan:
			s.ticker.Stop()
			log.Println("Currency rate scheduler stopped")
			return
		}
	}
}

// executeJob calls updateCurrencyRates and logs result
func (s *CurrencyRateScheduler) executeJob() {
	if err := s.updateCurrencyRates(); err != nil {
		log.Printf("Error updating currency rates: %v", err)
	} else {
		log.Printf("Currency rates updated successfully at %s UTC", time.Now().UTC().Format("2006-01-02 15:04:05"))
	}
}

// updateCurrencyRates fetches the latest currency rates and stores them in the database
func (s *CurrencyRateScheduler) updateCurrencyRates() error {
	rates, err := currency.GetCBRRates()
	if err != nil {
		return fmt.Errorf("failed to get CBR rates: %w", err)
	}

	var dbRates []storage.CurrencyRate
	currentDate := time.Now().Truncate(24 * time.Hour)

	for code, valute := range rates.Valute {
		dbRates = append(dbRates, storage.CurrencyRate{
			Date:         currentDate,
			CurrencyCode: code,
			CurrencyName: valute.Name,
			Nominal:      valute.Nominal,
			Value:        valute.Value,
			Previous:     valute.Previous,
		})
	}

	if err := s.db.SaveCurrencyRates(dbRates); err != nil {
		return fmt.Errorf("failed to save currency rates to database: %w", err)
	}

	return nil
}

// RunImmediately executes the currency rate update job immediately
func (s *CurrencyRateScheduler) RunImmediately() error {
	return s.updateCurrencyRates()
}