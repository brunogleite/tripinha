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
	ScannedAt   time.Time `json:"scanned_at"`
}

// Product holds the relevant fields returned by the product lookup.
type Product struct {
	Name string
}

// ErrProductNotFound is returned when a barcode lookup finds no matching product.
var ErrProductNotFound = errors.New("product not found")

// Storer persists meal events.
type Storer interface {
	Save(ctx context.Context, e MealEvent) (MealEvent, error)
}

// ProductFetcher looks up a product by barcode.
type ProductFetcher interface {
	Fetch(ctx context.Context, barcode string) (Product, error)
}
