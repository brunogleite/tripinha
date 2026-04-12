package consent

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/brunogleite/tripinha/internal/auth"
)

// Handler serves consent-related HTTP endpoints.
type Handler struct {
	store Storer
}

// NewHandler creates a Handler using store.
func NewHandler(store Storer) *Handler {
	return &Handler{store: store}
}

type postRequest struct {
	Version    string `json:"version"`
	AcceptedAt string `json:"accepted_at"`
}

// Post handles POST /consent.
// Stores {user_id, version, accepted_at}; returns 204 on success.
func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Version == "" || req.AcceptedAt == "" {
		http.Error(w, "version and accepted_at are required", http.StatusBadRequest)
		return
	}

	acceptedAt, err := time.Parse(time.RFC3339, req.AcceptedAt)
	if err != nil {
		http.Error(w, "accepted_at must be RFC3339", http.StatusBadRequest)
		return
	}

	if err := h.store.Upsert(r.Context(), Record{
		UserID:     userID,
		Version:    req.Version,
		AcceptedAt: acceptedAt,
	}); err != nil {
		log.Printf("failed to upsert consent event: %v", err) // ✅ log the error, not the event
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
