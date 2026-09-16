-- LLM トレーニング評価（日次バッチ）を保存する。既存の故障予防アドバイス
-- （advice / load-report）とは独立の追加機能。それらは毎回再計算するが、こちらは
-- 1 日 1 回のバッチ結果を永続化する。period_type は "biweekly"（直近2週間）と
-- "bimonthly"（直近8週間・約2ヶ月）の2種類。

CREATE TABLE IF NOT EXISTS training_evaluations (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  period_type   TEXT NOT NULL CHECK (period_type IN ('biweekly','bimonthly')),
  evaluated_at  TEXT NOT NULL,              -- 評価実行時刻 (RFC3339, JST)
  period_from   TEXT NOT NULL,              -- 対象期間開始日 (YYYY-MM-DD)
  period_to     TEXT NOT NULL,              -- 対象期間終了日 (YYYY-MM-DD, 通常は当日)
  model         TEXT NOT NULL DEFAULT '',
  summary       TEXT NOT NULL DEFAULT '',
  details_json  TEXT NOT NULL DEFAULT '{}', -- {"strengths":[...],"concerns":[...],"suggestions":[...]}
  raw_response  TEXT NOT NULL DEFAULT '',   -- 監査用。API では非公開
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_training_evaluations_period_evaluated_at
  ON training_evaluations(period_type, evaluated_at DESC);
