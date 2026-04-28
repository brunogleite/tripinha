package meals

import (
	"context"
	"errors"
	"time"
)

// MealEvent represents a logged barcode scan.
type MealEvent struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"user_id"`
	Barcode     string    `json:"barcode"`
	ProductName string    `json:"product_name"`
	Ingredients []string  `json:"ingredients,omitempty"`
	ScannedAt   time.Time `json:"scanned_at"`
}

// Product holds the relevant fields returned by the product lookup.
type Product struct {
	Name        string
	Ingredients []string
}

// ErrProductNotFound is returned when a barcode lookup finds no matching product.
var ErrProductNotFound = errors.New("product not found")

// ErrFetchFailed is returned when the product fetcher fails for a non-NotFound reason.
var ErrFetchFailed = errors.New("fetch failed")

// Storer persists meal events.
type Storer interface {
	Save(ctx context.Context, e MealEvent) (MealEvent, error)
}

// ProductFetcher looks up a product by barcode.
type ProductFetcher interface {
	Fetch(ctx context.Context, barcode string) (Product, error)
}

// IngredientNormalizer maps raw ingredient strings to canonical form.
type IngredientNormalizer interface {
	Normalize(raw []string) (canonical []string, flagged []string)
}

// FlaggedLogger stores unrecognized ingredients for manual review.
// Errors are non-fatal; callers must not block on this.
type FlaggedLogger interface {
	LogFlagged(ctx context.Context, mealEventID int64, ingredients []string) error
}
