package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/brunogleite/tripinha/internal/auth"
	"github.com/brunogleite/tripinha/internal/consent"
)

func main() {
	_ = godotenv.Load() // env may already be set in production

	db, err := pgxpool.New(context.Background(), mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	consentStore := consent.NewStore(db)

	r := chi.NewRouter()

	// All routes require a valid Auth0 JWT.
	r.Group(func(r chi.Router) {
		r.Use(auth.NewMiddleware("https://"+mustEnv("AUTH0_DOMAIN")+"/", mustEnv("AUTH0_AUDIENCE")))

		// Consent — no prior consent required to record it.
		r.Post("/consent", consent.NewHandler(consentStore).Post)

		// Health data routes — consent required.
		r.Group(func(r chi.Router) {
			r.Use(consent.RequireConsent(consentStore))
			// Future: r.Post("/meals", mealsHandler.Post)
			// Future: r.Post("/symptoms", symptomsHandler.Post)
		})
	})

	addr := ":" + envOr("PORT", "8080")
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("env %s required", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
