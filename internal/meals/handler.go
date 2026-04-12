package meals

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/brunogleite/tripinha/internal/auth"
)

// Handler serves meal-related HTTP endpoints.
type Handler struct {
	fetcher ProductFetcher
	store   Storer
}

// NewHandler creates a Handler using fetcher and store.
func NewHandler(fetcher ProductFetcher, store Storer) *Handler {
	return &Handler{fetcher: fetcher, store: store}
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

	product, err := h.fetcher.Fetch(r.Context(), req.Barcode)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			http.Error(w, "no product found for barcode: "+req.Barcode, http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch product", http.StatusBadGateway)
		return
	}

	event, err := h.store.Save(r.Context(), MealEvent{
		UserID:      userID,
		Barcode:     req.Barcode,
		ProductName: product.Name,
		ScannedAt:   time.Now().UTC(),
	})
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event) //nolint:errcheck
}
