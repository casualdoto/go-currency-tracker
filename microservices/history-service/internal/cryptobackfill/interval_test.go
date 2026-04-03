package cryptobackfill

import "testing"

func TestIntervalForCalendarSpan(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{1, "1m"},
		{7, "15m"},
		{30, "1h"},
		{90, "4h"},
		{91, "1d"},
	}
	for _, tt := range tests {
		if got := IntervalForCalendarSpan(tt.days); got != tt.want {
			t.Errorf("IntervalForCalendarSpan(%d)=%q want %q", tt.days, got, tt.want)
		}
	}
}
