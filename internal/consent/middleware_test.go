package consent_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brunogleite/tripinha/internal/consent"
)

func TestRequireConsent(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Cycle 8: no user in context → 401
	t.Run("no authenticated user → 401", func(t *testing.T) {
		mw := consent.RequireConsent(&fakeStore{existsVal: true})
		r := httptest.NewRequest(http.MethodPost, "/meals", nil)
		w := newRecorder()
		mw(okHandler).ServeHTTP(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	// Cycle 9: no consent record → 403
	t.Run("no consent record → 403", func(t *testing.T) {
		mw := consent.RequireConsent(&fakeStore{existsVal: false})
		r := httptest.NewRequest(http.MethodPost, "/meals", nil)
		r = withUserID(r, "user-1")
		w := newRecorder()
		mw(okHandler).ServeHTTP(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	// Cycle 10: store error → 500
	t.Run("store error → 500", func(t *testing.T) {
		mw := consent.RequireConsent(&fakeStore{existsErr: errStore})
		r := httptest.NewRequest(http.MethodPost, "/meals", nil)
		r = withUserID(r, "user-1")
		w := newRecorder()
		mw(okHandler).ServeHTTP(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
	})

	// Cycle 11: consent exists → passes through to next handler
	t.Run("consent exists → 200", func(t *testing.T) {
		mw := consent.RequireConsent(&fakeStore{existsVal: true})
		r := httptest.NewRequest(http.MethodPost, "/meals", nil)
		r = withUserID(r, "user-1")
		w := newRecorder()
		mw(okHandler).ServeHTTP(w, r)
		assertStatus(t, w, http.StatusOK)
	})
}
