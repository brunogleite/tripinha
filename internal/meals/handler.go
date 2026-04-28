package meals

import (
	"encoding/json"
	"errors"
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

	event, err := h.svc.LogMeal(r.Context(), userID, req.Barcode)
	if err != nil {
		switch {
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "no product found for barcode: "+req.Barcode, http.StatusNotFound)
		case errors.Is(err, ErrFetchFailed):
			http.Error(w, "failed to fetch product", http.StatusBadGateway)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event) //nolint:errcheck
}
