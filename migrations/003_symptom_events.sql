CREATE TABLE IF NOT EXISTS symptom_events (
  id          SERIAL PRIMARY KEY,
  user_id     TEXT        NOT NULL,
  type        TEXT        NOT NULL,
  severity    SMALLINT    NOT NULL CHECK (severity BETWEEN 1 AND 5),
  occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS symptom_events_user_occurred
  ON symptom_events (user_id, occurred_at);
