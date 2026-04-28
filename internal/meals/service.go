package meals

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// Service orchestrates meal event creation.
type Service struct {
	fetcher    ProductFetcher
	store      Storer
	normalizer IngredientNormalizer
	flagged    FlaggedLogger
}

// NewService creates a Service.
func NewService(fetcher ProductFetcher, store Storer, normalizer IngredientNormalizer, flagged FlaggedLogger) *Service {
	return &Service{
		fetcher:    fetcher,
		store:      store,
		normalizer: normalizer,
		flagged:    flagged,
	}
}

// LogMeal fetches product, normalizes ingredients, persists the event, and logs flagged ingredients.
func (s *Service) LogMeal(ctx context.Context, userID, barcode string) (MealEvent, error) {
	product, err := s.fetcher.Fetch(ctx, barcode)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			return MealEvent{}, err
		}
		return MealEvent{}, fmt.Errorf("%w: %w", ErrFetchFailed, err)
	}

	canonical, flaggedIngredients := s.normalizer.Normalize(product.Ingredients)

	event, err := s.store.Save(ctx, MealEvent{
		UserID:      userID,
		Barcode:     barcode,
		ProductName: product.Name,
		Ingredients: canonical,
		ScannedAt:   time.Now().UTC(),
	})
	if err != nil {
		log.Printf("failed to save meal event: %v", err)
		return MealEvent{}, fmt.Errorf("save meal event: %w", err)
	}

	if len(flaggedIngredients) > 0 {
		if err := s.flagged.LogFlagged(ctx, event.ID, flaggedIngredients); err != nil {
			log.Printf("log flagged ingredients for event %d: %v", event.ID, err)
		}
	}

	return event, nil
}
