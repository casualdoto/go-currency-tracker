package cbrbackfill

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseCBRRootDate(t *testing.T) {
	got, err := parseCBRRootDate("2024/03/15 11:30:00")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFetchDayWithFallback_usesPreviousArchiveDay(t *testing.T) {
	const usdJSON = `{"Date":"2026/03/28 11:30:00","Valute":{"U":{"CharCode":"USD","NumCode":"840","Nominal":1,"Name":"USD","Value":95.5,"Previous":95}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/2026/03/29/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/2026/03/28/") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(usdJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, http: srv.Client()}
	target := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	rates, src, err := c.FetchDayWithFallback(target)
	if err != nil {
		t.Fatal(err)
	}
	wantSrc := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	if !calendarDateUTC(src).Equal(wantSrc) {
		t.Fatalf("source day: got %v want %v", src, wantSrc)
	}
	var usd float64
	for _, r := range rates {
		if r.CurrencyCode == "USD" {
			usd = r.Value / float64(r.Nominal)
			break
		}
	}
	if usd != 95.5 {
		t.Fatalf("USD rate: got %v", usd)
	}
}
