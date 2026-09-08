# CLAUDE.md

このリポジトリで作業する Claude Code（および他の AI エージェント）向けのガイダンス。
詳細な設計は [`docs/`](./docs/README.md) に分割してある。まずそこを読む。

## プロジェクト概要

Hermes Agent のスキル **training-tracker**（筋力トレーニング + スピンバイクの記録・故障予防アドバイス）を、
スタンドアロンの **Web アプリケーション** に作り変える。

やること（`memo` より）:

1. 既存スキル機能の Web アプリ化 — 記録の CRUD / メニューをプリセット（ルーティン）保持 / 一覧表示 / カレンダー表示 / ボリュームのグラフ表示
2. `frontend` / `backend` / `nginx` の 3 コンポーネントに分割し `docker-compose` でインフラ設計
3. Hermes Agent と情報をやり取りするための API 提供

現状は **設計フェーズ（未実装）**。アプリのコードはまだない。

## リポジトリ構成

### 現状
```
training-record/
├── CLAUDE.md
├── memo                     # 発注内容メモ（日本語・原文）
├── docs/                    # 設計ドキュメント（このファイルの詳細版）
└── skill/                   # 移行元の Hermes Agent 資産（参照用・編集しない）
    ├── training.tgz
    └── .hermes/
        ├── home/.hermes/training-logs/training.db    # 既存データ（移行元）
        └── skills/
            ├── productivity/training-tracker/        # ★ 移行対象の本体
            │   ├── SKILL.md
            │   ├── scripts/training_db.py            # ドメインモデルと操作の正
            │   ├── scripts/training_load_analysis.py # Volume Load / ACWR / TRIMP
            │   └── references/                       # フォームガイド・スプリット分析
            └── mlops/training/                       # 参考資料（LLMファインチューニング系）。
                                                      # 直接の移行対象ではない
```

### 目標（実装後）
```
training-record/
├── CLAUDE.md / docs/
├── compose.yaml / compose.dev.yaml / .env.example
├── frontend/   # Next.js（1 compose サービス）
├── backend/    # Go REST API（1 compose サービス）
├── nginx/      # リバースプロキシ（1 compose サービス）
└── skill/      # 既存資産（不変）
```

## アーキテクチャ（要点）

| サービス | 技術 | 役割 |
|----------|------|------|
| `nginx` | nginx | 唯一の公開エンドポイント。`/` → frontend、`/api` → backend。TLS 終端 |
| `frontend` | Next.js (App Router) + TS | UI。記録 CRUD / 一覧 / カレンダー / ボリュームグラフ。backend は直接叩かず nginx 経由 |
| `backend` | Go (`net/http`) | REST API。ドメインロジックと永続化。frontend と Hermes Agent の共通バックエンド |

- 永続化: **backend 内 SQLite + named volume**（既存 `training.db` を初回に一度だけ取り込む。再同期なし）。Postgres 化は非スコープ
- 配置: **自宅 LAN 内・外部公開なし・Web UI のログイン認証なし**（ネットワークを信頼）。nginx は HTTP :80、TLS は任意
- 認証: `/api/*` は静的 API キー（`Authorization: Bearer`、`/api/health` 除く）。ブラウザは backend を直接叩かず Next.js サーバ側がキーを付与。Hermes は自分でキーを保持
- 詳細 → [docs/architecture.md](./docs/architecture.md)、決定事項一覧 → [docs/README.md](./docs/README.md)

## 設計ドキュメント

| ドキュメント | 内容 |
|--------------|------|
| [docs/plan.md](./docs/plan.md) | Web アプリ化プランとフェーズ分割（Phase 0〜6） |
| [docs/architecture.md](./docs/architecture.md) | コンポーネント構成・リクエストフロー・技術選定 |
| [docs/data-model.md](./docs/data-model.md) | DB スキーマ・既存 `training.db` からの移行 |
| [docs/api.md](./docs/api.md) | REST API 仕様（frontend / Hermes 共通、CLI との対応表） |
| [docs/infrastructure.md](./docs/infrastructure.md) | docker-compose・nginx・Dockerfile・環境変数 |
| [docs/domain.md](./docs/domain.md) | ドメイン用語・分析ロジック・落とし穴 |

## ドメインの正

- スキーマ: `skill/.hermes/skills/productivity/training-tracker/scripts/training_db.py` の `cmd_init`
  （`strength_sessions` / `exercises` / `spin_sessions` / `routine_snapshots` の 4 テーブル）
- 分析ロジック: `scripts/training_load_analysis.py`（Volume Load / ACWR / TRIMP）
- ワークフロー仕様: `SKILL.md`
- 実績（`exercises`）と ルーティン（`routine_snapshots`）は**別管理**。混同しない
- 用語・落とし穴の一覧は [docs/domain.md](./docs/domain.md)

## 開発コマンド

未整備（実装時に追記）。想定:
- `docker compose -f compose.yaml -f compose.dev.yaml up --build` — dev 一式
- frontend: `npm run dev` / `npm run build` / `npm run lint`
- backend: `go run ./cmd/server` / `go test ./...` / `go vet ./...`
- `make lint` / `make test` / `make build` — CI 相当をローカルで（ホスティング未定のため）

## 規約

- ドキュメント・UI 文言・コミットメッセージ本文は日本語で可（ドメインが日本語）
- `skill/` 配下は**移行元の参照資産**。原則編集しない
- 破壊的操作（既存 `training.db` の変更、`skill/` の削除等）は事前確認
- 日付は `YYYY-MM-DD` 文字列、タイムゾーンは `Asia/Tokyo` 固定
- API の JSON キーは camelCase、DB は snake_case（境界で変換）
- CLAUDE.md は肥大化させない。詳細は `docs/` に置き、ここは索引に留める
</content>
