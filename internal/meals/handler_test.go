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

	// Cycle 1: no authenticated user → 401
	t.Run("no authenticated user → 401", func(t *testing.T) {
		h := meals.NewHandler(&fakeFetcher{}, &fakeMealStore{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	// Cycle 2: invalid JSON body → 400
	t.Run("invalid JSON body → 400", func(t *testing.T) {
		h := meals.NewHandler(&fakeFetcher{}, &fakeMealStore{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString("not-json"))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 3: empty barcode → 400
	t.Run("empty barcode → 400", func(t *testing.T) {
		h := meals.NewHandler(&fakeFetcher{}, &fakeMealStore{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(`{"barcode":""}`))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 4: product not found → 404 with barcode in error message
	t.Run("product not found → 404 with barcode in message", func(t *testing.T) {
		h := meals.NewHandler(&fakeFetcher{err: meals.ErrProductNotFound}, &fakeMealStore{})
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
		h := meals.NewHandler(&fakeFetcher{err: errUpstream}, &fakeMealStore{})
		r := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadGateway)
	})

	// Cycle 6: store save error → 500
	t.Run("store error → 500", func(t *testing.T) {
		h := meals.NewHandler(
			&fakeFetcher{product: meals.Product{Name: "Nutella"}},
			&fakeMealStore{err: errUpstream},
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
		h := meals.NewHandler(
			&fakeFetcher{product: meals.Product{Name: "Nutella"}},
			store,
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
}
