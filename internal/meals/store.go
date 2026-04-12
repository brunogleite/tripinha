package meals

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles meal event persistence.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by db.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Save inserts a MealEvent and returns it with the DB-assigned ID and scanned_at.
func (s *Store) Save(ctx context.Context, e MealEvent) (MealEvent, error) {
	err := s.db.QueryRow(ctx,
		`INSERT INTO meal_events (user_id, barcode, product_name, scanned_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		e.UserID, e.Barcode, e.ProductName, e.ScannedAt,
	).Scan(&e.ID)
	return e, err
}
