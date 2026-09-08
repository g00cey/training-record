-- 0002_add_presets.sql
-- 単一ルーティン（routine_snapshots の最新スナップショット）を、名前付き複数
-- プリセットに拡張する。スキーマ変更のみ。データ投入・分類は起動時の Go fixup
-- ensureRoutinePresets が行う（fresh DB / 既存 DB を同じ経路で処理でき冪等）。

CREATE TABLE IF NOT EXISTS presets (
  name       TEXT PRIMARY KEY,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE routine_snapshots ADD COLUMN preset TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_rs_preset ON routine_snapshots(preset, date);
