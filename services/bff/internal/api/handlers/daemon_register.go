package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ramonehamilton/mtga-bff/internal/api/middleware"
)

// daemonRegisterResponse is the JSON body returned on a successful registration.
type daemonRegisterResponse struct {
	Token    string `json:"token"`
	DaemonID string `json:"daemon_id"`
}

// DaemonRegisterHandler handles daemon registration requests.
type DaemonRegisterHandler struct {
	jwtSecret string
}

// NewDaemonRegisterHandler returns a handler that issues daemon JWTs signed
// with jwtSecret.
func NewDaemonRegisterHandler(jwtSecret string) *DaemonRegisterHandler {
	return &DaemonRegisterHandler{jwtSecret: jwtSecret}
}

// Register handles POST /api/daemon/register.
//
// The endpoint must be protected by the APIKeyAuth middleware so that the
// authenticated user ID is available in the request context. The user_id
// embedded in the issued JWT is always derived from that context value —
// it is never accepted from the request body — to prevent privilege escalation.
//
// A new daemon_id UUID is allocated, a JWT (HS256, 30-day expiry) is signed,
// and the token is returned. The token must then be sent as
// "Authorization: Bearer <token>" on all /v1/ingest/events requests.
func (h *DaemonRegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Derive the user ID from the verified API-key context — never from the body.
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if strings.TrimSpace(h.jwtSecret) == "" {
		log.Println("[DaemonRegisterHandler] DAEMON_JWT_SECRET is not configured")
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	daemonID := uuid.NewString()

	token, err := middleware.IssueDaemonJWT(h.jwtSecret, userID, daemonID)
	if err != nil {
		log.Printf("[DaemonRegisterHandler] IssueDaemonJWT: %v", err)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := daemonRegisterResponse{
		Token:    token,
		DaemonID: daemonID,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[DaemonRegisterHandler] encode response: %v", err)
	}
}
