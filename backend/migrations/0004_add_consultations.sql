CREATE TABLE IF NOT EXISTS consultations (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  goal         TEXT NOT NULL,
  context_json TEXT NOT NULL DEFAULT '{}',
  created_at   TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS consultation_messages (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  consultation_id INTEGER NOT NULL REFERENCES consultations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content         TEXT NOT NULL,
  created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_consultations_updated_at
  ON consultations(updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_consultation_messages_consultation_id
  ON consultation_messages(consultation_id, id ASC);
