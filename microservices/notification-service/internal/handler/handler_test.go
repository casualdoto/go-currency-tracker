package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── stub store ───────────────────────────────────────────────────────────────

type stubStore struct {
	subscribeCBRErr    error
	unsubscribeCBRErr  error
	getCBRSubs         []string
	getCBRSubsErr      error
	subscribeCryptoErr error
	unsubCryptoErr     error
	getCryptoSubs      []string
	getCryptoSubsErr   error
}

func (s *stubStore) SubscribeCBR(_ context.Context, _ int64, _ string) error {
	return s.subscribeCBRErr
}
func (s *stubStore) UnsubscribeCBR(_ context.Context, _ int64, _ string) error {
	return s.unsubscribeCBRErr
}
func (s *stubStore) GetCBRSubscriptions(_ context.Context, _ int64) ([]string, error) {
	return s.getCBRSubs, s.getCBRSubsErr
}
func (s *stubStore) SubscribeCrypto(_ context.Context, _ int64, _ string) error {
	return s.subscribeCryptoErr
}
func (s *stubStore) UnsubscribeCrypto(_ context.Context, _ int64, _ string) error {
	return s.unsubCryptoErr
}
func (s *stubStore) GetCryptoSubscriptions(_ context.Context, _ int64) ([]string, error) {
	return s.getCryptoSubs, s.getCryptoSubsErr
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func post(t *testing.T, fn http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	fn(rr, req)
	return rr
}

func get(t *testing.T, fn http.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	fn(rr, req)
	return rr
}

// ─── SubscribeCBR ─────────────────────────────────────────────────────────────

func TestSubscribeCBR_success(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.SubscribeCBR, "/notifications/cbr/subscribe", `{"telegram_id":123,"value":"USD"}`)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestSubscribeCBR_invalidBody(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.SubscribeCBR, "/notifications/cbr/subscribe", `not json`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "invalid body" {
		t.Errorf("expected error 'invalid body', got %q", resp["error"])
	}
}

func TestSubscribeCBR_storeError(t *testing.T) {
	h := New(&stubStore{subscribeCBRErr: errors.New("redis down")})
	rr := post(t, h.SubscribeCBR, "/notifications/cbr/subscribe", `{"telegram_id":123,"value":"USD"}`)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ─── UnsubscribeCBR ───────────────────────────────────────────────────────────

func TestUnsubscribeCBR_success(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.UnsubscribeCBR, "/notifications/cbr/unsubscribe", `{"telegram_id":123,"value":"USD"}`)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestUnsubscribeCBR_invalidBody(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.UnsubscribeCBR, "/notifications/cbr/unsubscribe", `{bad}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestUnsubscribeCBR_storeError(t *testing.T) {
	h := New(&stubStore{unsubscribeCBRErr: errors.New("redis down")})
	rr := post(t, h.UnsubscribeCBR, "/notifications/cbr/unsubscribe", `{"telegram_id":123,"value":"USD"}`)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ─── ListCBRSubscriptions ─────────────────────────────────────────────────────

func TestListCBRSubscriptions_success(t *testing.T) {
	h := New(&stubStore{getCBRSubs: []string{"USD", "EUR"}})
	rr := get(t, h.ListCBRSubscriptions, "/notifications/cbr/subscriptions?telegram_id=123")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var subs []string
	json.NewDecoder(rr.Body).Decode(&subs)
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(subs))
	}
}

func TestListCBRSubscriptions_missingParam(t *testing.T) {
	h := New(&stubStore{})
	rr := get(t, h.ListCBRSubscriptions, "/notifications/cbr/subscriptions")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListCBRSubscriptions_invalidParam(t *testing.T) {
	h := New(&stubStore{})
	rr := get(t, h.ListCBRSubscriptions, "/notifications/cbr/subscriptions?telegram_id=abc")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListCBRSubscriptions_storeError(t *testing.T) {
	h := New(&stubStore{getCBRSubsErr: errors.New("redis down")})
	rr := get(t, h.ListCBRSubscriptions, "/notifications/cbr/subscriptions?telegram_id=123")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestListCBRSubscriptions_empty(t *testing.T) {
	h := New(&stubStore{getCBRSubs: []string{}})
	rr := get(t, h.ListCBRSubscriptions, "/notifications/cbr/subscriptions?telegram_id=123")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var subs []string
	json.NewDecoder(rr.Body).Decode(&subs)
	if len(subs) != 0 {
		t.Errorf("expected empty list, got %d items", len(subs))
	}
}

// ─── SubscribeCrypto ──────────────────────────────────────────────────────────

func TestSubscribeCrypto_success(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.SubscribeCrypto, "/notifications/crypto/subscribe", `{"telegram_id":123,"value":"BTCUSDT"}`)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestSubscribeCrypto_invalidBody(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.SubscribeCrypto, "/notifications/crypto/subscribe", `not json`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSubscribeCrypto_storeError(t *testing.T) {
	h := New(&stubStore{subscribeCryptoErr: errors.New("redis down")})
	rr := post(t, h.SubscribeCrypto, "/notifications/crypto/subscribe", `{"telegram_id":123,"value":"BTCUSDT"}`)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ─── UnsubscribeCrypto ────────────────────────────────────────────────────────

func TestUnsubscribeCrypto_success(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.UnsubscribeCrypto, "/notifications/crypto/unsubscribe", `{"telegram_id":123,"value":"BTCUSDT"}`)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestUnsubscribeCrypto_invalidBody(t *testing.T) {
	h := New(&stubStore{})
	rr := post(t, h.UnsubscribeCrypto, "/notifications/crypto/unsubscribe", `{bad}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestUnsubscribeCrypto_storeError(t *testing.T) {
	h := New(&stubStore{unsubCryptoErr: errors.New("redis down")})
	rr := post(t, h.UnsubscribeCrypto, "/notifications/crypto/unsubscribe", `{"telegram_id":123,"value":"BTCUSDT"}`)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ─── ListCryptoSubscriptions ──────────────────────────────────────────────────

func TestListCryptoSubscriptions_success(t *testing.T) {
	h := New(&stubStore{getCryptoSubs: []string{"BTCUSDT", "ETHUSDT"}})
	rr := get(t, h.ListCryptoSubscriptions, "/notifications/crypto/subscriptions?telegram_id=123")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var subs []string
	json.NewDecoder(rr.Body).Decode(&subs)
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(subs))
	}
}

func TestListCryptoSubscriptions_missingParam(t *testing.T) {
	h := New(&stubStore{})
	rr := get(t, h.ListCryptoSubscriptions, "/notifications/crypto/subscriptions")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListCryptoSubscriptions_invalidParam(t *testing.T) {
	h := New(&stubStore{})
	rr := get(t, h.ListCryptoSubscriptions, "/notifications/crypto/subscriptions?telegram_id=xyz")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestListCryptoSubscriptions_storeError(t *testing.T) {
	h := New(&stubStore{getCryptoSubsErr: errors.New("redis down")})
	rr := get(t, h.ListCryptoSubscriptions, "/notifications/crypto/subscriptions?telegram_id=123")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
