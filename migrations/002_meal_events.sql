CREATE TABLE IF NOT EXISTS meal_events (
  id           SERIAL PRIMARY KEY,
  user_id      TEXT        NOT NULL,
  barcode      TEXT        NOT NULL,
  product_name TEXT        NOT NULL,
  scanned_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS meal_events_user_scanned
  ON meal_events (user_id, scanned_at);
