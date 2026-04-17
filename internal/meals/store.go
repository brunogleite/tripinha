package meals

import (
	"context"
	"fmt"

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

// Save inserts a MealEvent (with its canonical ingredients) and returns it with the DB-assigned ID.
// Ingredients are stored in meal_ingredients; this runs in a single transaction.
func (s *Store) Save(ctx context.Context, e MealEvent) (MealEvent, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return MealEvent{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	err = tx.QueryRow(ctx,
		`INSERT INTO meal_events (user_id, barcode, product_name, scanned_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		e.UserID, e.Barcode, e.ProductName, e.ScannedAt,
	).Scan(&e.ID)
	if err != nil {
		return MealEvent{}, fmt.Errorf("insert meal event: %w", err)
	}

	if len(e.Ingredients) > 0 {
		_, err = tx.Exec(ctx,
			`INSERT INTO meal_ingredients (meal_event_id, ingredient_name)
			 SELECT $1, unnest($2::text[])`,
			e.ID, e.Ingredients,
		)
		if err != nil {
			return MealEvent{}, fmt.Errorf("insert ingredients: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return MealEvent{}, fmt.Errorf("commit tx: %w", err)
	}
	return e, nil
}

// LogFlagged inserts unrecognized ingredients into flagged_ingredients for manual review.
// Errors are logged by the caller but must not block the HTTP response.
func (s *Store) LogFlagged(ctx context.Context, mealEventID int64, ingredients []string) error {
	if len(ingredients) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO flagged_ingredients (meal_event_id, raw_ingredient)
		 SELECT $1, unnest($2::text[])`,
		mealEventID, ingredients,
	)
	if err != nil {
		return fmt.Errorf("log flagged ingredients: %w", err)
	}
	return nil
}
