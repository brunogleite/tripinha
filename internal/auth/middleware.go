package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type contextKey string

// UserIDKey is the context key for the authenticated user ID.
const UserIDKey contextKey = "user_id"

// customClaims is an empty set of custom JWT claims.
type customClaims struct{}

func (c *customClaims) Validate(_ context.Context) error { return nil }

// NewMiddleware returns an HTTP middleware that validates Auth0 JWTs.
// issuer is the full issuer URL, e.g. "https://your-tenant.auth0.com/".
// Tokens missing or invalid → 401. Valid tokens expose the sub claim via UserID.
func NewMiddleware(issuer, audience string) func(http.Handler) http.Handler {
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		panic("auth: invalid issuer URL: " + err.Error())
	}

	provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	v, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{audience},
		validator.WithCustomClaims(func() validator.CustomClaims { return &customClaims{} }),
	)
	if err != nil {
		panic("auth: failed to create JWT validator: " + err.Error())
	}

	m := jwtmiddleware.New(
		v.ValidateToken,
		jwtmiddleware.WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		}),
	)

	return func(next http.Handler) http.Handler {
		return m.CheckJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw := r.Context().Value(jwtmiddleware.ContextKey{}); raw != nil {
				if vc, ok := raw.(*validator.ValidatedClaims); ok {
					ctx := context.WithValue(r.Context(), UserIDKey, vc.RegisteredClaims.Subject)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// UserID returns the authenticated user's ID from ctx, or empty string if absent.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(UserIDKey).(string)
	return id
}
