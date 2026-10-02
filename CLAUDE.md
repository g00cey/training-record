# CLAUDE.md

このリポジトリで作業する Claude Code（および他の AI エージェント）向けのガイダンス。
詳細な設計は [`docs/`](./docs/README.md) に分割してある。まずそこを読む。

## プロジェクト概要

筋力トレーニング + スピンバイクの記録・故障予防アドバイスを扱う、スタンドアロンの **Web アプリケーション**。
旧来は CLI スキル（ローカル SQLite を直接操作する Python スクリプト）として配布されていたものを、
REST API を持つ Web アプリに作り変えた。

やること（`memo` より）:

1. 旧 CLI 機能の Web アプリ化 — 記録の CRUD / メニューをプリセット保持 / 一覧表示 / カレンダー表示 / ボリュームのグラフ表示
2. `frontend` / `backend` / `nginx` の 3 コンポーネントに分割し `docker-compose` でインフラ設計
3. 任意の外部クライアントと情報をやり取りするための REST API 提供

**現状: Phase 0〜8 実装済み・独立検証済み・運用中（2026-09-10 移行完了、2026-10 Phase 8 完了）**。
進捗の詳細は [docs/plan.md](./docs/plan.md)。

## リポジトリ構成

```
training-record/
├── CLAUDE.md / memo / docs/
├── compose.yaml / compose.dev.yaml / .env.example / Makefile
├── frontend/       # Next.js (App Router, standalone)（1 compose サービス）
├── backend/        # Go REST API（net/http + modernc.org/sqlite）（1 compose サービス）
├── nginx/          # リバースプロキシ（1 compose サービス）
└── llm/            # 画像抽出 / LLM トレーニング評価サービス（別 compose プロジェクト・外部 compose network 経由で接続）
```

## アーキテクチャ（要点）

| サービス | 技術 | 役割 |
|----------|------|------|
| `nginx` | nginx | 唯一の公開エンドポイント。`/api` → backend（API キー必須）、`/bff` と `/` → frontend。HTTP :80（TLS なし） |
| `frontend` | Next.js (App Router) + TS | UI。記録 CRUD / 一覧 / カレンダー / ボリュームグラフ / プリセット / アドバイス。ブラウザは `/bff/*`（Next.js Route Handler）だけを叩き、サーバ側で API キーを付与して backend へ中継 |
| `backend` | Go (`net/http` + `modernc.org/sqlite`) | REST API。ドメインロジックと永続化。frontend と任意の外部クライアントの共通バックエンド |
| `llm/` | Python stdlib HTTP サーバー | 画像抽出（`/extract-spin`）・LLM トレーニング評価（`/evaluate-training`）を提供。別 compose プロジェクト |

- 永続化: **backend 内 SQLite + named volume**（`./bootstrap/training.db` が存在すれば初回に一度だけ取り込む。再同期なし）。Postgres 化は非スコープ
- 配置: **自宅 LAN 内・外部公開なし・Web UI のログイン認証なし**（ネットワークを信頼）。nginx は HTTP :80、TLS なし
- 認証: `/api/*` は静的 API キー（`Authorization: Bearer`、`/api/health` 除く）。ブラウザは `/api` を使わず `/bff`（Next.js サーバがキー付与）。外部クライアントは自分でキーを保持し `/api` を直接叩く
- 詳細 → [docs/architecture.md](./docs/architecture.md)、決定事項一覧 → [docs/README.md](./docs/README.md)

## 設計ドキュメント

| ドキュメント | 内容 |
|--------------|------|
| [docs/plan.md](./docs/plan.md) | フェーズ分割と実装状況（Phase 0〜8） |
| [docs/architecture.md](./docs/architecture.md) | コンポーネント構成・リクエストフロー・技術選定 |
| [docs/data-model.md](./docs/data-model.md) | DB スキーマ（4 テーブル + presets + profile + training_evaluations）・`./bootstrap/training.db` からの任意インポート・0002/0003 |
| [docs/api.md](./docs/api.md) | REST API 仕様（frontend / 外部クライアント共通、CLI との対応表、実装で確定した挙動） |
| [docs/infrastructure.md](./docs/infrastructure.md) | docker-compose・nginx・Dockerfile・環境変数 |
| [docs/domain.md](./docs/domain.md) | ドメイン用語・分析ロジック・落とし穴 |

## ドメインの正

- **実装済み**: スキーマ・分析ロジックは `backend/`（`internal/store` / `internal/service` / `migrations/`）にある。
  設計の出典:
  - スキーマ: 旧 CLI スキルの `cmd_init`（`strength_sessions` / `exercises` / `spin_sessions` / `routine_snapshots`）を移植、Web 版で `presets` / `profile` / `schema_migrations` / `training_evaluations` を追加、`routine_snapshots` に `preset` 列（0002）
  - 分析ロジック: 旧 CLI スキルの解析スクリプト（Volume Load / ACWR / TRIMP）→ `backend/internal/service/analytics.go`・`advice.go`
  - 故障予防アドバイス: 旧 CLI スキルの「故障予防アドバイス」節と参考資料 → `backend/internal/service/advice.go`
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
- 破壊的操作（既存 SQLite の変更、`./bootstrap/training.db` の削除等）は事前確認
- 日付は `YYYY-MM-DD` 文字列、タイムゾーンは `Asia/Tokyo` 固定
- API の JSON キーは camelCase、DB は snake_case（境界で変換）
- CLAUDE.md は肥大化させない。詳細は `docs/` に置き、ここは索引に留める
</content>
