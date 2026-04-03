package cryptobackfill

// IntervalForCalendarSpan matches monolith logic (monolith/internal/api/crypto_handlers.go).
func IntervalForCalendarSpan(days int) string {
	switch {
	case days <= 1:
		return "1m"
	case days <= 7:
		return "15m"
	case days <= 30:
		return "1h"
	case days <= 90:
		return "4h"
	default:
		return "1d"
	}
}
