package meals_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/brunogleite/tripinha/internal/auth"
	"github.com/brunogleite/tripinha/internal/meals"
)

// fakeFetcher is a test double for meals.ProductFetcher.
type fakeFetcher struct {
	product meals.Product
	err     error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) (meals.Product, error) {
	return f.product, f.err
}

// fakeMealStore is a test double for meals.Storer.
type fakeMealStore struct {
	saved meals.MealEvent
	err   error
}

func (f *fakeMealStore) Save(_ context.Context, e meals.MealEvent) (meals.MealEvent, error) {
	if f.err != nil {
		return meals.MealEvent{}, f.err
	}
	e.ID = 42 // simulate DB-assigned ID
	f.saved = e
	return e, nil
}

var errUpstream = errors.New("upstream error")

// withUserID injects userID into r's context (simulates auth middleware).
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
