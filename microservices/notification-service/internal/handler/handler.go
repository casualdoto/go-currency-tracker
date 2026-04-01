package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/casualdoto/go-currency-tracker/microservices/notification-service/internal/store"
)

type Handler struct {
	store *store.RedisStore
}

func New(s *store.RedisStore) *Handler {
	return &Handler{store: s}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type subRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Value      string `json:"value"` // currency code or symbol
}

func (h *Handler) SubscribeCBR(w http.ResponseWriter, r *http.Request) {
	var req subRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.SubscribeCBR(context.Background(), req.TelegramID, req.Value); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnsubscribeCBR(w http.ResponseWriter, r *http.Request) {
	var req subRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.UnsubscribeCBR(context.Background(), req.TelegramID, req.Value); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListCBRSubscriptions(w http.ResponseWriter, r *http.Request) {
	tidStr := r.URL.Query().Get("telegram_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid telegram_id"})
		return
	}
	subs, err := h.store.GetCBRSubscriptions(context.Background(), tid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) SubscribeCrypto(w http.ResponseWriter, r *http.Request) {
	var req subRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.SubscribeCrypto(context.Background(), req.TelegramID, req.Value); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnsubscribeCrypto(w http.ResponseWriter, r *http.Request) {
	var req subRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.store.UnsubscribeCrypto(context.Background(), req.TelegramID, req.Value); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListCryptoSubscriptions(w http.ResponseWriter, r *http.Request) {
	tidStr := r.URL.Query().Get("telegram_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid telegram_id"})
		return
	}
	subs, err := h.store.GetCryptoSubscriptions(context.Background(), tid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, subs)
}
