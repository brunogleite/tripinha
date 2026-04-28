package meals_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brunogleite/tripinha/internal/meals"
)

func TestHandler_Post(t *testing.T) {
	validBody := `{"barcode":"3017620422003"}`
	nopNormalizer := meals.NewNormalizer(nil) // no-op: flags everything, existing tests don't check ingredients

	newHandler := func(fetcher meals.ProductFetcher, store meals.Storer, normalizer meals.IngredientNormalizer, flagged meals.FlaggedLogger) *meals.Handler {
		return meals.NewHandler(meals.NewService(fetcher, store, normalizer, flagged))
	}

	// Cycle 1: no authenticated user → 401
	t.Run("no authenticated user → 401", func(t *testing.T) {
		h := newHandler(&fakeFetcher{}, &fakeMealStore{}, nopNormalizer, &fakeFlaggedLogger{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	// Cycle 2: invalid JSON body → 400
	t.Run("invalid JSON body → 400", func(t *testing.T) {
		h := newHandler(&fakeFetcher{}, &fakeMealStore{}, nopNormalizer, &fakeFlaggedLogger{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString("not-json"))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 3: empty barcode → 400
	t.Run("empty barcode → 400", func(t *testing.T) {
		h := newHandler(&fakeFetcher{}, &fakeMealStore{}, nopNormalizer, &fakeFlaggedLogger{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(`{"barcode":""}`))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 4: product not found → 404 with barcode in error message
	t.Run("product not found → 404 with barcode in message", func(t *testing.T) {
		h := newHandler(&fakeFetcher{err: meals.ErrProductNotFound}, &fakeMealStore{}, nopNormalizer, &fakeFlaggedLogger{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusNotFound)
		if !strings.Contains(w.Body.String(), "3017620422003") {
			t.Errorf("error body should mention the barcode; got: %q", w.Body.String())
		}
	})

	// Cycle 5: product fetcher non-NotFound error → 502 Bad Gateway
	t.Run("upstream fetch error → 502", func(t *testing.T) {
		h := newHandler(&fakeFetcher{err: errUpstream}, &fakeMealStore{}, nopNormalizer, &fakeFlaggedLogger{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadGateway)
	})

	// Cycle 6: store save error → 500
	t.Run("store error → 500", func(t *testing.T) {
		h := newHandler(
			&fakeFetcher{product: meals.Product{Name: "Nutella"}},
			&fakeMealStore{err: errUpstream},
			nopNormalizer,
			&fakeFlaggedLogger{},
		)
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
	})

	// Cycle 7: valid request → 201 with meal event JSON
	t.Run("valid request → 201 with meal event body", func(t *testing.T) {
		store := &fakeMealStore{}
		h := newHandler(
			&fakeFetcher{product: meals.Product{Name: "Nutella"}},
			store,
			nopNormalizer,
			&fakeFlaggedLogger{},
		)
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-42")
		w := newRecorder()
		h.Post(w, r)

		assertStatus(t, w, http.StatusCreated)

		var got meals.MealEvent
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.ID != 42 {
			t.Errorf("ID: got %d, want 42", got.ID)
		}
		if got.UserID != "user-42" {
			t.Errorf("UserID: got %q, want %q", got.UserID, "user-42")
		}
		if got.Barcode != "3017620422003" {
			t.Errorf("Barcode: got %q, want %q", got.Barcode, "3017620422003")
		}
		if got.ProductName != "Nutella" {
			t.Errorf("ProductName: got %q, want %q", got.ProductName, "Nutella")
		}
		if got.ScannedAt.IsZero() {
			t.Error("ScannedAt should not be zero")
		}
	})

	// Cycle 16: handler normalizes ingredients; canonical saved on event; flagged logged without blocking
	t.Run("canonical ingredients saved; flagged logged; request not blocked", func(t *testing.T) {
		store := &fakeMealStore{}
		logger := &fakeFlaggedLogger{}
		normalizer := meals.NewNormalizer([]string{"Sugar", "Palm Oil"})
		h := newHandler(
			&fakeFetcher{product: meals.Product{
				Name:        "Nutella",
				Ingredients: []string{"Sugar", "palm oil", "xylitol"},
			}},
			store,
			normalizer,
			logger,
		)
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-42")
		w := newRecorder()
		h.Post(w, r)

		assertStatus(t, w, http.StatusCreated)

		wantIngredients := []string{"Sugar", "Palm Oil"}
		if len(store.saved.Ingredients) != len(wantIngredients) {
			t.Fatalf("Ingredients len: got %d, want %d; got %v", len(store.saved.Ingredients), len(wantIngredients), store.saved.Ingredients)
		}
		for i, want := range wantIngredients {
			if store.saved.Ingredients[i] != want {
				t.Errorf("Ingredients[%d]: got %q, want %q", i, store.saved.Ingredients[i], want)
			}
		}
		if len(logger.flagged) != 1 || logger.flagged[0] != "xylitol" {
			t.Errorf("flagged: got %v, want [xylitol]", logger.flagged)
		}
	})
}
