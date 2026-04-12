package consent_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brunogleite/tripinha/internal/consent"
)

func TestHandler_Post(t *testing.T) {
	validBody := `{"version":"v1","accepted_at":"2026-04-12T10:00:00Z"}`

	// Cycle 1: no user in context → 401
	t.Run("no authenticated user → 401", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(validBody))
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	// Cycle 2: invalid JSON body → 400
	t.Run("invalid JSON body → 400", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString("not-json"))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 3: missing version → 400
	t.Run("missing version → 400", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(`{"accepted_at":"2026-04-12T10:00:00Z"}`))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 4: missing accepted_at → 400
	t.Run("missing accepted_at → 400", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(`{"version":"v1"}`))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 5: accepted_at not RFC3339 → 400
	t.Run("accepted_at not RFC3339 → 400", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(`{"version":"v1","accepted_at":"12-04-2026"}`))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Cycle 6: store error → 500
	t.Run("store error → 500", func(t *testing.T) {
		h := consent.NewHandler(&fakeStore{upsertErr: errStore})
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-1")
		w := newRecorder()
		h.Post(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
	})

	// Cycle 7: valid request → 204, record stored with correct fields
	t.Run("valid request → 204 and record stored", func(t *testing.T) {
		store := &fakeStore{}
		h := consent.NewHandler(store)
		r := httptest.NewRequest(http.MethodPost, "/consent", bytes.NewBufferString(validBody))
		r = withUserID(r, "user-42")
		w := newRecorder()
		h.Post(w, r)

		assertStatus(t, w, http.StatusNoContent)

		if store.upserted == nil {
			t.Fatal("Upsert not called")
		}
		if store.upserted.UserID != "user-42" {
			t.Errorf("UserID: got %q, want %q", store.upserted.UserID, "user-42")
		}
		if store.upserted.Version != "v1" {
			t.Errorf("Version: got %q, want %q", store.upserted.Version, "v1")
		}
		wantTime := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
		if !store.upserted.AcceptedAt.Equal(wantTime) {
			t.Errorf("AcceptedAt: got %v, want %v", store.upserted.AcceptedAt, wantTime)
		}
	})
}
