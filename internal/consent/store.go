package consent

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storer is the minimal interface for consent persistence.
// Accepted by Handler and RequireConsent so they can be tested without a real DB.
type Storer interface {
	Upsert(ctx context.Context, r Record) error
	Exists(ctx context.Context, userID string) (bool, error)
}

// Record represents a user's consent record.
type Record struct {
	UserID     string
	Version    string
	AcceptedAt time.Time
}

// Store handles consent persistence.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by db.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Upsert stores or updates a consent record.
func (s *Store) Upsert(ctx context.Context, r Record) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO consent (user_id, version, accepted_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET version = EXCLUDED.version,
		       accepted_at = EXCLUDED.accepted_at`,
		r.UserID, r.Version, r.AcceptedAt,
	)
	return err
}

// Exists reports whether a consent record exists for userID.
func (s *Store) Exists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM consent WHERE user_id = $1)`,
		userID,
	).Scan(&exists)
	return exists, err
}
