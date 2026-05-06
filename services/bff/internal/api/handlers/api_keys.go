package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
	"github.com/ramonehamilton/mtga-bff/internal/storage/repository"
)

// apiKeyCreator is the subset of APIKeyRepository used by APIKeysHandler.
type apiKeyCreator interface {
	Create(ctx context.Context, userID int64, keyHash string) (*repository.APIKey, error)
}

// APIKeysHandler handles requests to create API keys.
type APIKeysHandler struct {
	repo apiKeyCreator
}

// NewAPIKeysHandler returns a handler backed by the given repository.
func NewAPIKeysHandler(repo apiKeyCreator) *APIKeysHandler {
	return &APIKeysHandler{repo: repo}
}

// createAPIKeyResponse is the JSON response for a successful key creation.
// The plaintext key is shown exactly once and is never stored.
type createAPIKeyResponse struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAPIKey handles POST /api/keys.
//
// The route must be protected by APIKeyAuth middleware, which validates the
// daemon's API key and stores the user_id in the request context.  The
// handler reads the user identity exclusively from context — the X-User-ID
// header placeholder has been removed.
//
// On success it returns 201 with the plaintext key in the response body.
// The key is shown only once; it is not recoverable from the server.
func (h *APIKeysHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate 32 random bytes → hex-encoded plaintext key (64 chars).
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		log.Printf("[APIKeysHandler] rand.Read: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	plaintextKey := hex.EncodeToString(rawBytes)

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextKey), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[APIKeysHandler] bcrypt: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	apiKey, err := h.repo.Create(r.Context(), userID, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			writeJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		log.Printf("[APIKeysHandler] Create: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := createAPIKeyResponse{
		Key:       plaintextKey,
		CreatedAt: apiKey.CreatedAt,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[APIKeysHandler] encode response: %v", err)
	}
}

// writeJSONError writes a JSON error body with the given HTTP status.
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	body, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(body)
}
