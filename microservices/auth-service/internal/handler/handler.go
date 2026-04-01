package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/casualdoto/go-currency-tracker/microservices/auth-service/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db        *storage.PostgresDB
	jwtSecret string
}

func New(db *storage.PostgresDB, jwtSecret string) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"email and password required"})
		return
	}

	existing, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, errorResponse{"email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to hash password"})
		return
	}

	user, err := h.db.CreateUser(req.Email, string(hash))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to create user"})
		return
	}

	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to generate token"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if err := h.db.SaveSession(token, user.ID, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to save session"})
		return
	}

	writeJSON(w, http.StatusCreated, tokenResponse{Token: token})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid request body"})
		return
	}

	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"database error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid credentials"})
		return
	}

	token, err := h.generateToken(user.ID, user.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to generate token"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if err := h.db.SaveSession(token, user.ID, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to save session"})
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{Token: token})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"missing token"})
		return
	}
	if err := h.db.DeleteSession(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"failed to invalidate session"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"missing token"})
		return
	}

	valid, userID, err := h.db.IsSessionValid(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"database error"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"invalid or expired token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

func (h *Handler) generateToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(h.jwtSecret))
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
