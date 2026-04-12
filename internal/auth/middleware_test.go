package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gopkg.in/go-jose/go-jose.v2"
	josejwt "gopkg.in/go-jose/go-jose.v2/jwt"

	"github.com/brunogleite/tripinha/internal/auth"
)

const testAudience = "https://api.tripinha.test"

// testEnv holds a local JWKS server and signing key for JWT tests.
type testEnv struct {
	server  *httptest.Server
	privKey *rsa.PrivateKey
	kid     string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	kid := "test-kid-1"
	env := &testEnv{privKey: privKey, kid: kid}

	env.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			// OIDC discovery document — the auth0 provider fetches this first
			// to locate the jwks_uri before fetching keys.
			doc := map[string]string{"jwks_uri": env.server.URL + "/.well-known/jwks.json"}
			if err := json.NewEncoder(w).Encode(doc); err != nil {
				t.Errorf("encode discovery doc: %v", err)
			}
		case "/.well-known/jwks.json":
			jwk := jose.JSONWebKey{
				Key:       &privKey.PublicKey,
				KeyID:     kid,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			}
			if err := json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}); err != nil {
				t.Errorf("encode JWKS: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(env.server.Close)

	return env
}

// sign creates a signed JWT with the given subject, audience, and expiry.
func (e *testEnv) sign(t *testing.T, sub, aud string, expiry time.Time) string {
	t.Helper()

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: e.privKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", e.kid),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	cl := josejwt.Claims{
		Subject:  sub,
		Issuer:   e.server.URL + "/",
		Audience: josejwt.Audience{aud},
		Expiry:   josejwt.NewNumericDate(expiry),
		IssuedAt: josejwt.NewNumericDate(time.Now()),
	}

	raw, err := josejwt.Signed(sig).Claims(cl).CompactSerialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return raw
}

// Cycle 12: missing token → 401
func TestNewMiddleware_MissingToken(t *testing.T) {
	env := newTestEnv(t)
	mw := auth.NewMiddleware(env.server.URL+"/", testAudience)

	reached := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
	if reached {
		t.Error("inner handler should not be reached without token")
	}
}

// Cycle 13: expired token → 401
func TestNewMiddleware_ExpiredToken(t *testing.T) {
	env := newTestEnv(t)
	mw := auth.NewMiddleware(env.server.URL+"/", testAudience)

	token := env.sign(t, "user-expired", testAudience, time.Now().Add(-time.Hour))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
}

// Cycle 14: wrong audience → 401
func TestNewMiddleware_WrongAudience(t *testing.T) {
	env := newTestEnv(t)
	mw := auth.NewMiddleware(env.server.URL+"/", testAudience)

	token := env.sign(t, "user-1", "https://wrong-audience.test", time.Now().Add(time.Hour))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
}

// Cycle 15: valid token → 200, sub claim stored in context as user ID
func TestNewMiddleware_ValidToken(t *testing.T) {
	env := newTestEnv(t)
	mw := auth.NewMiddleware(env.server.URL+"/", testAudience)

	token := env.sign(t, "user-123", testAudience, time.Now().Add(time.Hour))

	var gotUserID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = auth.UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if gotUserID != "user-123" {
		t.Errorf("user ID: got %q, want %q", gotUserID, "user-123")
	}
}

// Cycle 16: UserID returns empty string when context has no user
func TestUserID_EmptyWhenAbsent(t *testing.T) {
	got := auth.UserID(context.Background())
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
