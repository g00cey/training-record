-- 0001_init.sql
-- 移行元スキーマの正: skill/.hermes/skills/productivity/training-tracker/scripts/training_db.py の cmd_init
-- Web 版はこれを踏襲し、カラム追加のみ行う（strength_sessions / spin_sessions に updated_at、profile 新規）。

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS strength_sessions (
    id         INTEGER PRIMARY KEY,
    date       TEXT NOT NULL UNIQUE,
    notes      TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS exercises (
    id         INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES strength_sessions(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    weight     REAL,
    reps       INTEGER NOT NULL,
    sets       INTEGER DEFAULT 1,
    notes      TEXT DEFAULT '',
    sort_order INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS spin_sessions (
    id               INTEGER PRIMARY KEY,
    date             TEXT NOT NULL UNIQUE,
    duration_minutes INTEGER NOT NULL,
    avg_heart_rate   INTEGER,
    max_heart_rate   INTEGER,
    rpe              INTEGER CHECK(rpe IS NULL OR (rpe >= 1 AND rpe <= 10)),
    distance_km      REAL,
    notes            TEXT DEFAULT '',
    created_at       TEXT DEFAULT (datetime('now')),
    updated_at       TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS routine_snapshots (
    id            INTEGER PRIMARY KEY,
    date          TEXT NOT NULL,
    exercise_name TEXT NOT NULL,
    weight        REAL,
    reps          INTEGER NOT NULL,
    sets          INTEGER DEFAULT 1,
    sort_order    INTEGER DEFAULT 0,
    created_at    TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS profile (
    id            INTEGER PRIMARY KEY CHECK(id = 1),
    bodyweight_kg REAL    NOT NULL DEFAULT 86.0,
    height_cm     REAL    NOT NULL DEFAULT 170.0,
    max_hr_est    INTEGER NOT NULL DEFAULT 180,
    updated_at    TEXT DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO profile (id, bodyweight_kg, height_cm, max_hr_est, updated_at)
VALUES (1, 86.0, 170.0, 180, datetime('now'));

CREATE INDEX IF NOT EXISTS idx_ex_name    ON exercises(name);
CREATE INDEX IF NOT EXISTS idx_ex_session ON exercises(session_id);
CREATE INDEX IF NOT EXISTS idx_rs_date    ON routine_snapshots(date);
CREATE INDEX IF NOT EXISTS idx_rs_ex      ON routine_snapshots(exercise_name);
CREATE INDEX IF NOT EXISTS idx_ss_date    ON strength_sessions(date);
CREATE INDEX IF NOT EXISTS idx_spin_date  ON spin_sessions(date);
