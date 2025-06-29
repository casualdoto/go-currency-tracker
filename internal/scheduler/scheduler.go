// Package scheduler provides task scheduling functionality
package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/casualdoto/go-currency-tracker/internal/currency"
	"github.com/casualdoto/go-currency-tracker/internal/storage"
)

// CurrencyRateScheduler is responsible for scheduling currency rate updates
type CurrencyRateScheduler struct {
	db           *storage.PostgresDB
	stopChan     chan struct{}
	isRunning    bool
	dailyJobTime time.Time
}

// NewCurrencyRateScheduler creates a new scheduler for currency rate updates
func NewCurrencyRateScheduler(db *storage.PostgresDB, hourUTC, minuteUTC int) *CurrencyRateScheduler {
	// Create a time.Time for the job time (today at specified hour:minute UTC)
	now := time.Now().UTC()
	jobTime := time.Date(now.Year(), now.Month(), now.Day(), hourUTC, minuteUTC, 0, 0, time.UTC)

	// If the time has already passed today, schedule for tomorrow
	if now.After(jobTime) {
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
		return
	}

	s.isRunning = true
	go s.run()
}

// Stop stops the scheduler
func (s *CurrencyRateScheduler) Stop() {
	if !s.isRunning {
		return
	}

	s.stopChan <- struct{}{}
	s.isRunning = false
}

// run is the main loop for the scheduler
func (s *CurrencyRateScheduler) run() {
	log.Printf("Currency rate scheduler started. Next update at %s UTC", s.dailyJobTime.Format("15:04"))

	// Calculate time until next run
	timeUntilNextRun := s.dailyJobTime.Sub(time.Now().UTC())
	if timeUntilNextRun < 0 {
		// If negative, add 24 hours to schedule for tomorrow
		timeUntilNextRun += 24 * time.Hour
	}

	timer := time.NewTimer(timeUntilNextRun)

	for {
		select {
		case <-timer.C:
			// Execute the job
			if err := s.updateCurrencyRates(); err != nil {
				log.Printf("Error updating currency rates: %v", err)
			} else {
				log.Printf("Currency rates updated successfully at %s UTC", time.Now().UTC().Format("2006-01-02 15:04:05"))
			}

			// Schedule next run (24 hours later)
			s.dailyJobTime = s.dailyJobTime.Add(24 * time.Hour)
			timer.Reset(24 * time.Hour)
			log.Printf("Next currency rate update scheduled for %s UTC", s.dailyJobTime.Format("2006-01-02 15:04:05"))

		case <-s.stopChan:
			// Stop the timer to prevent resource leaks
			if !timer.Stop() {
				<-timer.C
			}
			log.Println("Currency rate scheduler stopped")
			return
		}
	}
}

// updateCurrencyRates fetches the latest currency rates and stores them in the database
func (s *CurrencyRateScheduler) updateCurrencyRates() error {
	// Get current rates from CBR
	rates, err := currency.GetCBRRates()
	if err != nil {
		return fmt.Errorf("failed to get CBR rates: %w", err)
	}

	// Convert to database format
	var dbRates []storage.CurrencyRate
	currentDate := time.Now().Truncate(24 * time.Hour) // Set to beginning of day

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

	// Save to database
	if err := s.db.SaveCurrencyRates(dbRates); err != nil {
		return fmt.Errorf("failed to save currency rates to database: %w", err)
	}

	return nil
}

// RunImmediately executes the currency rate update job immediately
func (s *CurrencyRateScheduler) RunImmediately() error {
	return s.updateCurrencyRates()
}
