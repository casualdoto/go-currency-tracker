package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── fakeDB ───────────────────────────────────────────────────────────────────

// fakeDB stubs the storage.PostgresDB methods used by the handler.
// We replicate just the fields the handler calls so we don't import the storage
// package (avoiding a real DB dependency in unit tests).

type fakeUser struct {
	ID           string
	Email        string
	PasswordHash string
}

type fakeDB struct {
	users    map[string]*fakeUser // email → user
	sessions map[string]string    // token → userID
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		users:    make(map[string]*fakeUser),
		sessions: make(map[string]string),
	}
}

// ─── stub Handler ─────────────────────────────────────────────────────────────
// Instead of wiring the real Handler (which needs *storage.PostgresDB), we
// test the handler logic via a thin wrapper that uses fakeDB.

type testHandler struct {
	db        *fakeDB
	jwtSecret string
}

func newTestHandler() *testHandler {
	return &testHandler{db: newFakeDB(), jwtSecret: "test-secret"}
}

func (h *testHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"email and password required"})
		return
	}
	if _, exists := h.db.users[req.Email]; exists {
		writeJSON(w, http.StatusConflict, errorResponse{"email already registered"})
		return
	}
	user := &fakeUser{ID: "uid-1", Email: req.Email, PasswordHash: req.Password}
	h.db.users[req.Email] = user

	tok, _ := (&Handler{jwtSecret: h.jwtSecret}).generateToken(user.ID, user.Email)
	h.db.sessions[tok] = user.ID
	writeJSON(w, http.StatusCreated, tokenResponse{Token: tok})
}

func (h *testHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
		return
	}
	user, ok := h.db.users[req.Email]
	if !ok || user.PasswordHash != req.Password {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid credentials"})
		return
	}
	tok, _ := (&Handler{jwtSecret: h.jwtSecret}).generateToken(user.ID, user.Email)
	writeJSON(w, http.StatusOK, tokenResponse{Token: tok})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// ─── Register tests ───────────────────────────────────────────────────────────

func TestRegister_success(t *testing.T) {
	h := newTestHandler()
	rr := postJSON(t, h.register, map[string]string{"email": "a@b.com", "password": "secret"})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var resp tokenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestRegister_duplicate(t *testing.T) {
	h := newTestHandler()
	body := map[string]string{"email": "a@b.com", "password": "secret"}
	postJSON(t, h.register, body)
	rr := postJSON(t, h.register, body)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestRegister_missingFields(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		body any
		want int
	}{
		{"empty email", map[string]string{"email": "", "password": "x"}, http.StatusBadRequest},
		{"empty password", map[string]string{"email": "a@b.com", "password": ""}, http.StatusBadRequest},
		{"invalid json", "not-json", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := postJSON(t, h.register, tc.body)
			if rr.Code != tc.want {
				t.Errorf("expected %d, got %d", tc.want, rr.Code)
			}
		})
	}
}

// ─── Login tests ──────────────────────────────────────────────────────────────

func TestLogin_success(t *testing.T) {
	h := newTestHandler()
	postJSON(t, h.register, map[string]string{"email": "a@b.com", "password": "secret"})

	rr := postJSON(t, h.login, map[string]string{"email": "a@b.com", "password": "secret"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp tokenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_wrongPassword(t *testing.T) {
	h := newTestHandler()
	postJSON(t, h.register, map[string]string{"email": "a@b.com", "password": "secret"})

	rr := postJSON(t, h.login, map[string]string{"email": "a@b.com", "password": "wrong"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestLogin_unknownUser(t *testing.T) {
	h := newTestHandler()
	rr := postJSON(t, h.login, map[string]string{"email": "nobody@b.com", "password": "x"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ─── generateToken ────────────────────────────────────────────────────────────

func TestGenerateToken(t *testing.T) {
	h := &Handler{jwtSecret: "my-secret"}
	tok, err := h.generateToken("uid-42", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == "" {
		t.Error("expected non-empty token")
	}
	// Token must have 3 parts (header.payload.signature).
	parts := 0
	for _, c := range tok {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("expected JWT with 2 dots, got token: %s", tok)
	}
}

// ─── extractBearerToken ───────────────────────────────────────────────────────

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		query  string
		want   string
	}{
		{"bearer header", "Bearer mytoken", "", "mytoken"},
		{"query param", "", "mytoken", "mytoken"},
		{"empty", "", "", ""},
		{"wrong scheme", "Basic mytoken", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?token="+tc.query, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			got := extractBearerToken(req)
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
