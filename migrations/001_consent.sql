CREATE TABLE IF NOT EXISTS consent (
  user_id     TEXT PRIMARY KEY,
  version     TEXT NOT NULL,
  accepted_at TIMESTAMPTZ NOT NULL
);
