package symptoms_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brunogleite/tripinha/internal/symptoms"
)

func TestHandler_Post(t *testing.T) {
	validBody := `{"type":"bloating","severity":3,"occurred_at":"2026-04-12T10:00:00Z"}`

	// Cycle 1: no authenticated user → 401
	t.Run("no authenticated user → 401", func(t *testing.T) {
		h := symptoms.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString(validBody))
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	// Cycle 2: invalid JSON body → 400
	t.Run("invalid JSON body → 400", func(t *testing.T) {
		h := symptoms.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString("not-json"))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 3: empty type → 400
	t.Run("empty type → 400", func(t *testing.T) {
		h := symptoms.NewHandler(&fakeStore{})
		body := `{"type":"","severity":3,"occurred_at":"2026-04-12T10:00:00Z"}`
		r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString(body))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 6: valid request → 201 with symptom event body
	t.Run("valid request → 201 with event body", func(t *testing.T) {
		store := &fakeStore{}
		h := symptoms.NewHandler(store)
		r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-42")
		w := newRecorder()
		h.Post(w, r)

		assertStatus(t, w, http.StatusCreated)

		var got symptoms.SymptomEvent
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.ID != 99 {
			t.Errorf("ID: got %d, want 99", got.ID)
		}
		if got.UserID != "user-42" {
			t.Errorf("UserID: got %q, want %q", got.UserID, "user-42")
		}
		if got.Type != "bloating" {
			t.Errorf("Type: got %q, want %q", got.Type, "bloating")
		}
		if got.Severity != 3 {
			t.Errorf("Severity: got %d, want 3", got.Severity)
		}
		if got.OccurredAt.IsZero() {
			t.Error("OccurredAt should not be zero")
		}
	})

	// Cycle 5: store error → 500
	t.Run("store error → 500", func(t *testing.T) {
		h := symptoms.NewHandler(&fakeStore{err: errDB})
		r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
	})

	// Cycle 4: severity out of range → 422
	for _, tc := range []struct {
		name     string
		severity int
	}{
		{"severity zero", 0},
		{"severity too high", 6},
		{"severity negative", -1},
	} {
		t.Run(tc.name+" → 422", func(t *testing.T) {
			h := symptoms.NewHandler(&fakeStore{})
			body := fmt.Sprintf(`{"type":"bloating","severity":%d,"occurred_at":"2026-04-12T10:00:00Z"}`, tc.severity)
			r := httptest.NewRequest(http.MethodPost, "/symptoms", bytes.NewBufferString(body))
			r = withUserID(r, "user-1")
			w := newRecorder()
			h.Post(w, r)
			assertStatus(t, w, http.StatusUnprocessableEntity)
		})
	}
}
