# CLAUDE.md

このリポジトリで作業する Claude Code（および他の AI エージェント）向けのガイダンス。
詳細な設計は [`docs/`](./docs/README.md) に分割してある。まずそこを読む。

## プロジェクト概要

Hermes Agent のスキル **training-tracker**（筋力トレーニング + スピンバイクの記録・故障予防アドバイス）を、
スタンドアロンの **Web アプリケーション** に作り変える。

やること（`memo` より）:

1. 既存スキル機能の Web アプリ化 — 記録の CRUD / メニューをプリセット保持 / 一覧表示 / カレンダー表示 / ボリュームのグラフ表示
2. `frontend` / `backend` / `nginx` の 3 コンポーネントに分割し `docker-compose` でインフラ設計
3. Hermes Agent と情報をやり取りするための API 提供

**現状: Phase 0〜6 実装済み・独立検証済み・移行完了（2026-09-10）。Phase 7A（スピン画像登録・Web 側）実装済み、7B（Hermes 側抽出 API）は依頼文送付待ち。Phase 8（LLM トレーニング評価・日次バッチ）実装済み**。Hermes は API 版スキル（v2）に切替済み。
進捗の詳細は [docs/plan.md](./docs/plan.md)、移行の記録は [docs/hermes-integration.md](./docs/hermes-integration.md)。

## リポジトリ構成

```
training-record/
├── CLAUDE.md / memo / docs/
├── compose.yaml / compose.dev.yaml / .env.example / Makefile
├── frontend/       # Next.js (App Router, standalone)（1 compose サービス）
├── backend/        # Go REST API（net/http + modernc.org/sqlite）（1 compose サービス）
├── nginx/          # リバースプロキシ（1 compose サービス）
├── hermes-skill/   # Hermes Agent 向け新スキル v2（API 版）: SKILL.md + scripts/training_api.py + references/
└── skill/          # 移行元の Hermes Agent 資産（参照専用・不変・2026-09-07 で凍結）
    └── .hermes/…/training-logs/training.db          # 移行元データ（backend の DB と一致）
        …/skills/productivity/training-tracker/       # 旧スキル（training_db.py / training_load_analysis.py / references）
```

## アーキテクチャ（要点）

| サービス | 技術 | 役割 |
|----------|------|------|
| `nginx` | nginx | 唯一の公開エンドポイント。`/api` → backend（Hermes 用）、`/bff` と `/` → frontend。HTTP :80（TLS なし） |
| `frontend` | Next.js (App Router) + TS | UI。記録 CRUD / 一覧 / カレンダー / ボリュームグラフ / プリセット / アドバイス。ブラウザは `/bff/*`（Next.js Route Handler）だけを叩き、サーバ側で API キーを付与して backend へ中継 |
| `backend` | Go (`net/http` + `modernc.org/sqlite`) | REST API。ドメインロジックと永続化。frontend と Hermes Agent の共通バックエンド |

- 永続化: **backend 内 SQLite + named volume**（既存 `training.db` を初回に一度だけ取り込む。再同期なし）。Postgres 化は非スコープ
- 配置: **自宅 LAN 内・外部公開なし・Web UI のログイン認証なし**（ネットワークを信頼）。nginx は HTTP :80、TLS なし
- 認証: `/api/*` は静的 API キー（`Authorization: Bearer`、`/api/health` 除く）。ブラウザは `/api` を使わず `/bff`（Next.js サーバがキー付与）。Hermes は自分でキーを保持し `/api` を直接叩く
- 詳細 → [docs/architecture.md](./docs/architecture.md)、決定事項一覧 → [docs/README.md](./docs/README.md)

## 設計ドキュメント

| ドキュメント | 内容 |
|--------------|------|
| [docs/plan.md](./docs/plan.md) | フェーズ分割と実装状況（Phase 0〜6） |
| [docs/architecture.md](./docs/architecture.md) | コンポーネント構成・リクエストフロー・技術選定 |
| [docs/data-model.md](./docs/data-model.md) | DB スキーマ（4 テーブル + presets + profile）・`training.db` からの移行・0002 |
| [docs/api.md](./docs/api.md) | REST API 仕様（frontend / Hermes 共通、CLI との対応表、実装で確定した挙動） |
| [docs/infrastructure.md](./docs/infrastructure.md) | docker-compose・nginx・Dockerfile・環境変数 |
| [docs/domain.md](./docs/domain.md) | ドメイン用語・分析ロジック・落とし穴 |
| [docs/hermes-integration.md](./docs/hermes-integration.md) | Hermes 連携（Phase 5/6）・修正依頼文・カットオーバー記録 |

## ドメインの正

- **実装済み**: スキーマ・分析ロジックは `backend/`（`internal/store` / `internal/service` / `migrations/`）にある。
  以下は移行元（設計の出典・凍結）:
  - スキーマ: `skill/…/training-tracker/scripts/training_db.py` の `cmd_init`（`strength_sessions` / `exercises` / `spin_sessions` / `routine_snapshots`）＋ Web 版で `presets` / `profile` / `schema_migrations` を追加、`routine_snapshots` に `preset` 列（0002）
  - 分析ロジック: `scripts/training_load_analysis.py`（Volume Load / ACWR / TRIMP）→ `backend/internal/service/analytics.go`・`advice.go`
  - ワークフロー仕様: 旧 `SKILL.md`（新版は `hermes-skill/training-tracker/SKILL.md`）
- 実績（`exercises`）と プリセット（`routine_snapshots`）は**別管理**。混同しない
- 用語・落とし穴の一覧は [docs/domain.md](./docs/domain.md)

## 開発コマンド

- `make up` — 本番相当構成で起動（`docker compose up --build -d`、http://localhost/）
- `make dev` — 開発構成（`compose.yaml` + `compose.dev.yaml`、nginx はホスト :8081）
- `make down` / `make clean`（`down -v`・DB 初期化）/ `make db_backup`
- `make lint`（`go vet` + `npm run lint`）/ `make test`（`go test ./...`）/ `make build`
- backend 単体: `cd backend && go run ./cmd/server`（要 env `API_KEY` 等）/ `go test ./...`
- frontend 単体: `cd frontend && npm run dev` / `npm run build` / `npm run lint`

## 規約

- ドキュメント・UI 文言・コミットメッセージ本文は日本語で可（ドメインが日本語）
- `skill/` 配下は**移行元の参照資産**。原則編集しない
- 破壊的操作（既存 `training.db` の変更、`skill/` の削除等）は事前確認
- 日付は `YYYY-MM-DD` 文字列、タイムゾーンは `Asia/Tokyo` 固定
- API の JSON キーは camelCase、DB は snake_case（境界で変換）
- CLAUDE.md は肥大化させない。詳細は `docs/` に置き、ここは索引に留める
</content>
