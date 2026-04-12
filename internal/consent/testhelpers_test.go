package consent_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/brunogleite/tripinha/internal/auth"
	"github.com/brunogleite/tripinha/internal/consent"
)

// fakeStore is a test double for consent.Storer.
type fakeStore struct {
	upsertErr error
	upserted  *consent.Record

	existsVal bool
	existsErr error
}

func (f *fakeStore) Upsert(_ context.Context, r consent.Record) error {
	f.upserted = &r
	return f.upsertErr
}

func (f *fakeStore) Exists(_ context.Context, _ string) (bool, error) {
	return f.existsVal, f.existsErr
}

var errStore = errors.New("db error")

// withUserID injects userID into r's context (simulates auth middleware).
func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
	return r.WithContext(ctx)
}

// newRecorder wraps httptest.NewRecorder for brevity.
func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func assertStatus(t interface {
	Helper()
	Errorf(string, ...any)
}, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status: got %d, want %d", w.Code, want)
	}
}
