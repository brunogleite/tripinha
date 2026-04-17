-- Canonical ingredient dictionary (IBS-relevant seed data).
-- Requires human review before normalization is considered trustworthy (issue #5).
CREATE TABLE IF NOT EXISTS ingredients (
  id   SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

-- Normalized ingredients linked to a meal event.
CREATE TABLE IF NOT EXISTS meal_ingredients (
  id             SERIAL PRIMARY KEY,
  meal_event_id  INTEGER NOT NULL REFERENCES meal_events(id) ON DELETE CASCADE,
  ingredient_name TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS meal_ingredients_meal_event
  ON meal_ingredients (meal_event_id);

-- Unrecognized ingredients flagged for manual review.
-- Append-only; never blocks the request.
CREATE TABLE IF NOT EXISTS flagged_ingredients (
  id             SERIAL PRIMARY KEY,
  meal_event_id  INTEGER     NOT NULL REFERENCES meal_events(id) ON DELETE CASCADE,
  raw_ingredient TEXT        NOT NULL,
  flagged_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS flagged_ingredients_meal_event
  ON flagged_ingredients (meal_event_id);
