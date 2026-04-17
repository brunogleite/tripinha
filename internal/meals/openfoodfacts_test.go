// White-box test — package meals, so we can access the unexported test constructor.
package meals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// offTestServer creates a fake Open Food Facts server.
// handler writes a response for each incoming request.
func offTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// Cycle 8: successful lookup → returns Product with name
func TestOFFClient_Fetch_Success(t *testing.T) {
	srv := offTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/product/3017620422003.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  1,
			"product": map[string]any{"product_name": "Nutella"},
		})
	})

	c := newOFFClientWithBase(srv.URL, srv.Client())
	got, err := c.Fetch(t.Context(), "3017620422003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Nutella" {
		t.Errorf("Name: got %q, want %q", got.Name, "Nutella")
	}
}

// Cycle 9: status 0 → ErrProductNotFound
func TestOFFClient_Fetch_NotFound(t *testing.T) {
	srv := offTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": 0})
	})

	c := newOFFClientWithBase(srv.URL, srv.Client())
	_, err := c.Fetch(t.Context(), "0000000000000")
	if err != ErrProductNotFound {
		t.Errorf("got %v, want ErrProductNotFound", err)
	}
}

// Cycle 10: server returns non-200 → error (not ErrProductNotFound)
func TestOFFClient_Fetch_HTTPError(t *testing.T) {
	srv := offTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})

	c := newOFFClientWithBase(srv.URL, srv.Client())
	_, err := c.Fetch(t.Context(), "3017620422003")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err == ErrProductNotFound {
		t.Error("should not be ErrProductNotFound for HTTP error")
	}
}

// Cycle 11: server returns invalid JSON → error
func TestOFFClient_Fetch_BadJSON(t *testing.T) {
	srv := offTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	})

	c := newOFFClientWithBase(srv.URL, srv.Client())
	_, err := c.Fetch(t.Context(), "3017620422003")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Cycle 12: successful lookup with ingredients_text → Product.Ingredients split and trimmed
func TestOFFClient_Fetch_WithIngredients(t *testing.T) {
	srv := offTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": 1,
			"product": map[string]any{
				"product_name":     "Nutella",
				"ingredients_text": "Sugar, Palm Oil, Hazelnuts 13%",
			},
		})
	})

	c := newOFFClientWithBase(srv.URL, srv.Client())
	got, err := c.Fetch(t.Context(), "3017620422003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Sugar", "Palm Oil", "Hazelnuts 13%"}
	if len(got.Ingredients) != len(want) {
		t.Fatalf("Ingredients len: got %d, want %d; got %v", len(got.Ingredients), len(want), got.Ingredients)
	}
	for i, w := range want {
		if got.Ingredients[i] != w {
			t.Errorf("Ingredients[%d]: got %q, want %q", i, got.Ingredients[i], w)
		}
	}
}
