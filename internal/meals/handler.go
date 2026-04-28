package meals

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/brunogleite/tripinha/internal/auth"
)

// Handler serves meal-related HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Post handles POST /meals.
// Accepts {"barcode": string}, fetches product from Open Food Facts,
// stores the meal event, and returns 201 with the created event.
func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Barcode string `json:"barcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Barcode == "" {
		http.Error(w, "barcode is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("failed to save meal event: %v", err) // ✅ log the error, not the event
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(flaggedIngredients) > 0 {
		if err := h.flagged.LogFlagged(r.Context(), event.ID, flaggedIngredients); err != nil {
			log.Printf("log flagged ingredients for event %d: %v", event.ID, err)
		}
	}
	//until here

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event) //nolint:errcheck
}
