package symptoms_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/brunogleite/tripinha/internal/auth"
	"github.com/brunogleite/tripinha/internal/symptoms"
)

type fakeStore struct {
	saved symptoms.SymptomEvent
	err   error
}

func (f *fakeStore) Save(_ context.Context, e symptoms.SymptomEvent) (symptoms.SymptomEvent, error) {
	if f.err != nil {
		return symptoms.SymptomEvent{}, f.err
	}
	e.ID = 99 // simulate DB-assigned ID
	f.saved = e
	return e, nil
}

var errDB = errors.New("db error")

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
	return r.WithContext(ctx)
}

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
