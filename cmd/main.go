package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/brunogleite/tripinha/internal/auth"
	"github.com/brunogleite/tripinha/internal/consent"
	"github.com/brunogleite/tripinha/internal/meals"
	"github.com/brunogleite/tripinha/internal/symptoms"
)

func main() {
	_ = godotenv.Load() // env may already be set in production

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	db, err := pgxpool.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	authMiddleware, err := auth.NewMiddleware("https://"+mustEnv("AUTH0_DOMAIN")+"/", mustEnv("AUTH0_AUDIENCE"))
	if err != nil {
		log.Fatalf("%v", err)
	}

	consentStore := consent.NewStore(db)
	mealStore := meals.NewStore(db)
	mealHandler := meals.NewHandler(meals.NewOFFClient(), mealStore, meals.NewNormalizer(meals.IBSIngredients()), mealStore)
	symptomHandler := symptoms.NewHandler(symptoms.NewStore(db))

	r := chi.NewRouter()

	// All routes require a valid Auth0 JWT.
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		// Consent — no prior consent required to record it.
		r.Post("/consent", consent.NewHandler(consentStore).Post)

		// Health data routes — consent required.
		r.Group(func(r chi.Router) {
			r.Use(consent.RequireConsent(consentStore))
			r.Post("/meals", mealHandler.Post)
			r.Post("/symptoms", symptomHandler.Post)
		})
	})

	server := &http.Server{Addr: ":" + envOr("PORT", "8080"), Handler: r}

	go func() {
		log.Printf("listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	db.Close()
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
