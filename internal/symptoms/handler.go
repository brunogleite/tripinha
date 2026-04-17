package symptoms

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/brunogleite/tripinha/internal/auth"
)

// Handler serves symptom-related HTTP endpoints.
type Handler struct {
	store Storer
}

// NewHandler creates a Handler using store.
func NewHandler(store Storer) *Handler {
	return &Handler{store: store}
}

// Post handles POST /symptoms.
func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Type       string    `json:"type"`
		Severity   int       `json:"severity"`
		OccurredAt time.Time `json:"occurred_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}
	if req.Severity < 1 || req.Severity > 5 {
		http.Error(w, "severity must be between 1 and 5", http.StatusUnprocessableEntity)
		return
	}

	event, err := h.store.Save(r.Context(), SymptomEvent{
		UserID:     userID,
		Type:       req.Type,
		Severity:   req.Severity,
		OccurredAt: req.OccurredAt,
	})
	if err != nil {
		log.Printf("store.Save failed: user=%s type=%s err=%v", userID, req.Type, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event) //nolint:errcheck
}
