package symptoms

import (
	"context"
	"time"
)

// SymptomEvent represents a logged symptom.
type SymptomEvent struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	Type       string    `json:"type"`
	Severity   int       `json:"severity"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Storer persists symptom events.
type Storer interface {
	Save(ctx context.Context, e SymptomEvent) (SymptomEvent, error)
}
