# データモデルと移行

移行元スキーマの正: `skill/.hermes/skills/productivity/training-tracker/scripts/training_db.py` の `cmd_init`。
Web 版はこれを踏襲し、カラム追加のみ行う（意味は変えない）。

## テーブル定義（Web 版）

### strength_sessions — 筋トレの 1 日分
| カラム | 型 | 備考 |
|--------|----|----|
| `id` | INTEGER PK | |
| `date` | TEXT NOT NULL UNIQUE | `YYYY-MM-DD`。1 日 1 セッション |
| `notes` | TEXT DEFAULT '' | 例: `自重のみ` / `自重＋フリーウェイト` / `FWのみ; 自重のみ` |
| `created_at` | TEXT DEFAULT (datetime('now')) | |
| `updated_at` | TEXT DEFAULT (datetime('now')) | **追加**。更新時に touch |

### exercises — セッション内の各種目の実績
| カラム | 型 | 備考 |
|--------|----|----|
| `id` | INTEGER PK | |
| `session_id` | INTEGER NOT NULL | → `strength_sessions(id)` ON DELETE CASCADE |
| `name` | TEXT NOT NULL | 日本語種目名。表記ゆれ注意 |
| `weight` | REAL | 自重種目は `NULL`（0 ではない） |
| `reps` | INTEGER NOT NULL | |
| `sets` | INTEGER DEFAULT 1 | |
| `notes` | TEXT DEFAULT '' | |
| `sort_order` | INTEGER DEFAULT 0 | = 実施順 |

- 同名種目が複数行あり得る（「懸垂 10r」「懸垂 6r」）。統合しない。

### spin_sessions — スピンバイク
| カラム | 型 | 備考 |
|--------|----|----|
| `id` | INTEGER PK | |
| `date` | TEXT NOT NULL UNIQUE | |
| `duration_minutes` | INTEGER NOT NULL | |
| `avg_heart_rate` | INTEGER | |
| `max_heart_rate` | INTEGER | |
| `rpe` | INTEGER | CHECK 1–10 or NULL。未指定時は `round(avg_hr/max_hr*10)` で自動算出（現行踏襲） |
| `distance_km` | REAL | |
| `notes` | TEXT DEFAULT '' | 心拍ゾーン内訳の慣習フォーマットあり → [domain.md](./domain.md) |
| `created_at` / `updated_at` | TEXT | `updated_at` は**追加** |

### routine_snapshots — プリセット（ルーティン）の履歴
| カラム | 型 | 備考 |
|--------|----|----|
| `id` | INTEGER PK | |
| `date` | TEXT NOT NULL | スナップショット日。**UNIQUE ではない**（同日に複数種目行） |
| `exercise_name` | TEXT NOT NULL | ※ API では `name` に正規化 → [api.md](./api.md) |
| `weight` | REAL | |
| `reps` | INTEGER NOT NULL | |
| `sets` | INTEGER DEFAULT 1 | |
| `sort_order` | INTEGER DEFAULT 0 | |
| `created_at` | TEXT | |

- 最新 `date` の行群 = 「現在のルーティン」。変更のたびに新 `date` で全行を積み直す（部分更新でも新スナップショットを作る）。

### profile — ユーザプロフィール（**新規・単一行**）
| カラム | 型 | 既定 | 用途 |
|--------|----|------|------|
| `id` | INTEGER PK CHECK(id=1) | 1 | |
| `bodyweight_kg` | REAL | 86.0 | 自重種目の Volume Load 計算 |
| `height_cm` | REAL | 170.0 | BMI 表示 |
| `max_hr_est` | INTEGER | 180 | TRIMP 計算 |
| `updated_at` | TEXT | | |

現行はスクリプトにハードコード（`training_load_analysis.py` の `BODYWEIGHT_KG` 等）。テーブル化して API から編集可能にする。

### schema_migrations — マイグレーション管理（**新規**）
| `version` INTEGER PK | `applied_at` TEXT |

## インデックス（現行踏襲）

`idx_ex_name`, `idx_ex_session`, `idx_rs_date`, `idx_rs_ex`, `idx_ss_date`, `idx_spin_date`
＋ 追加: `idx_ss_date` は UNIQUE 制約で足りるが、範囲検索が多いので明示的に残す。

## 既存データの移行

移行元: `skill/.hermes/home/.hermes/training-logs/training.db`

| テーブル | 件数 | 期間 |
|----------|------|------|
| strength_sessions | 36 | 2026-07-03 〜 2026-09-07 |
| exercises | 559 | — |
| spin_sessions | 5 | 2026-07-11 〜 2026-07-22 |
| routine_snapshots | 41（= 2 スナップショット: `2026-07-03` 19 種目 / `2026-07-30` 22 種目） | — |

### 方式（旧スキルは停止済み前提・初回一度きり）
1. backend 起動時、`DB_PATH`（`/data/training.db`）が存在しなければ:
   - スキーマ用マイグレーションを version 1 から適用
   - `BOOTSTRAP_DB_PATH` が設定されていて実在すれば、その SQLite から 4 テーブルを `INSERT`（`id` も維持）
   - `profile` に既定行を投入
2. 既に `DB_PATH` があれば未適用マイグレーションのみ適用（データ移行はしない）
3. **再同期の仕組みは作らない**。旧 Hermes スキル（`training_db.py`）は Web 稼働と同時に運用停止する。
   取り込みをやり直したい場合は `training-data` volume を削除して再起動（`BOOTSTRAP_DB_PATH` から再取り込み）。

### 移行スクリプト
- `backend/cmd/migrate-legacy`（Go の小コマンド）または `make bootstrap`。
- ソース DB は読み取り専用でオープン。種目名の表記ゆれ正規化は**この段階では行わない**（別タスク）。
  - 既知のゆれ: `懸垂（窄握）`（実績 1 件）と `懸垂（宽握）`（ルーティン、字体は簡体 `宽`）。`SKILL.md` 本文では `窄握`/`宽握` が混在。移行時はそのまま取り込む。

## スキーマ変更の方針

- 連番 SQL ファイル `backend/migrations/0001_init.sql`, `0002_xxx.sql` …（`embed`）
- 破壊的変更（列削除・リネーム）は避け、追加中心
- `training.db` を直接いじらない。変更は必ずマイグレーション経由
</content>
