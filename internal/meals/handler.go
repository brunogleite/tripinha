package meals

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/brunogleite/tripinha/internal/auth"
)

// Handler serves meal-related HTTP endpoints.
type Handler struct {
	fetcher    ProductFetcher
	store      Storer
	normalizer IngredientNormalizer
	flagged    FlaggedLogger
}

// NewHandler creates a Handler.
func NewHandler(fetcher ProductFetcher, store Storer, normalizer IngredientNormalizer, flagged FlaggedLogger) *Handler {
	return &Handler{fetcher: fetcher, store: store, normalizer: normalizer, flagged: flagged}
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

	canonical, flaggedIngredients := h.normalizer.Normalize(product.Ingredients)

	event, err := h.store.Save(r.Context(), MealEvent{
		UserID:      userID,
		Barcode:     req.Barcode,
		ProductName: product.Name,
		Ingredients: canonical,
		ScannedAt:   time.Now().UTC(),
	})
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event) //nolint:errcheck
}
