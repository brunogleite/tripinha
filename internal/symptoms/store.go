package symptoms

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles symptom event persistence.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by db.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Save inserts a SymptomEvent and returns it with the DB-assigned ID.
func (s *Store) Save(ctx context.Context, e SymptomEvent) (SymptomEvent, error) {
	err := s.db.QueryRow(ctx,
		`INSERT INTO symptom_events (user_id, type, severity, occurred_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		e.UserID, e.Type, e.Severity, e.OccurredAt,
	).Scan(&e.ID)
	return e, err
}
