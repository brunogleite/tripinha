package consent

import (
	"net/http"

	"github.com/brunogleite/tripinha/internal/auth"
)

// RequireConsent blocks health data writes when no consent record exists for the user.
// Missing consent → 403.
func RequireConsent(store Storer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.UserID(r.Context())
			if userID == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ok, err := store.Exists(r.Context(), userID)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "consent required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
