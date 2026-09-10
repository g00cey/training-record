# アーキテクチャ

## コンポーネント

| サービス | 技術 | 公開 | 役割 |
|----------|------|------|------|
| `nginx` | nginx (alpine) | ホストの :80 / :443 | 唯一の公開エンドポイント。TLS 終端、`/api`（→ backend）と `/bff`・`/`（→ frontend）の振り分け |
| `frontend` | Next.js (App Router) + TypeScript | コンテナ内 :3000（nginx からのみ） | UI。ブラウザからは同一オリジンの `/bff/...`（Next.js Route Handler）だけを叩く。Route Handler がサーバ側で API キーを付与し backend へ転送 |
| `backend` | Go + `net/http` (Go 1.22 `ServeMux`) | コンテナ内 :8080（nginx からのみ） | REST API。ドメインロジックと永続化。frontend と Hermes Agent の共通バックエンド |

DB は backend コンテナ内の SQLite ファイル（named volume に永続化）。→ [data-model.md](./data-model.md)

## リクエストフロー

```
ブラウザ ──► nginx :80/:443
                │  location /        ──► frontend:3000   (Next.js 本体・静的アセット)
                │  location /bff/    ──► frontend:3000   (Next.js Route Handler /bff/[...path])
                │                            └─ サーバ側で Authorization: Bearer を付与
                │                               INTERNAL_API_BASE (http://backend:8080/api) へ転送
                │  location /api/    ──► backend:8080     (Go REST API・Hermes 用)
                                             │
                                             └──► SQLite (/data/training.db, volume: training-data)

Hermes Agent ──► nginx :80  /api/...  ──► backend:8080   (Hermes 自身が API キーを保持)
```

- frontend は backend を**直接参照しない**。ブラウザは同一オリジンの `/bff/*` のみを叩き、Next.js の Route Handler が `INTERNAL_API_BASE` + API キーで backend に中継する。
- ブラウザ公開用のベースパスは `/bff`（`NEXT_PUBLIC_API_BASE=/bff`）。`/api` は backend 直で、キーを持つ Hermes Agent 専用。
- この分離により API キーが意味を持つ（`/api` に到達するには キーが必須。ブラウザは `/api` を使わない）。

## 技術選定

> 実装済み。具体バージョン: frontend = Next.js 15.5.x（App Router, standalone）/ Node 24 / Tailwind v3 /
> SWR / React Hook Form + Zod / Recharts。backend = Go 1.25 / `modernc.org/sqlite` /
> prod は `gcr.io/distroless/static-debian12:nonroot`。

### frontend
- **Next.js (App Router) + TypeScript** — 一覧/カレンダー/グラフを持つ SPA 寄りの画面。SSR も使える
- **Tailwind CSS** + 自前コンポーネント（UI ライブラリは入れない）
- **グラフ**: Recharts（Volume Load 推移、種目別推移、ACWR）
- **カレンダー**: 月グリッドを自前実装（依存を増やさない）。日セルに種別バッジ
- **データ取得**: `fetch` + SWR（軽いキャッシュ・再検証）
- **フォーム**: React Hook Form + Zod（種目リストの動的行、バリデーション）

### backend
- **Go 標準 `net/http` + `ServeMux`（Go 1.22 のパターンルーティング）**。ルータ依存を足さない。必要になれば `chi`
- **DB アクセス**: `database/sql` + `modernc.org/sqlite`（Pure Go、CGO 不要 → scratch イメージが作れる）
- **マイグレーション**: `embed` した連番 SQL を起動時に適用する自前ランナー（`schema_migrations` テーブル管理）。→ [data-model.md](./data-model.md)
- **レイヤ**: `handler`（HTTP） → `service`（ドメインロジック・分析計算） → `store`（SQL）
- **設定**: 環境変数のみ（`DB_PATH`, `API_KEY`, `PORT`, `TZ`, `BOOTSTRAP_DB_PATH`）

### 分析ロジックの移植元
- `skill/.hermes/skills/productivity/training-tracker/scripts/training_db.py` … CRUD・ルーティン・集計 SQL
- `skill/.hermes/skills/productivity/training-tracker/scripts/training_load_analysis.py` … Volume Load / ACWR / TRIMP
- これらを Go の `service` 層に移植する。SQL はほぼそのまま流用可。

## 認証・ネットワーク

**前提: 自宅 LAN 内・外部公開なし・Web UI にログイン認証は設けない**（ネットワークを信頼）。

- `/api/*` は `Authorization: Bearer <API_KEY>` を必須（`/api/health` を除く）。LAN 越しの Hermes 呼び出しに対する軽い防御（defense-in-depth）として残す
- ブラウザは backend を直接叩かない。フロー:
  - ブラウザ → nginx `/bff/*` → frontend の Next.js Route Handler（`app/bff/[...path]`） → `INTERNAL_API_BASE` + API キーを付与して backend
  - Hermes Agent（別マシン） → nginx `/api/*` → backend（Hermes 自身が API キーを保持）
- API キーはブラウザに出さない（Next.js サーバ側の env `API_KEY` のみ）。nginx でのキー付与はしない（付与すると LAN 内の誰でも `/api` を素通しできてしまうため）
- ブラウザ経路（`/bff`）と Hermes 経路（`/api`）を別パスにするのが要点。nginx `/api/` を frontend に向けると キーが付かず 401 になる（初回実装で踏んだ落とし穴）
- CORS 設定は不要（frontend は同一オリジン、Hermes はサーバ間呼び出し）
- TLS: 必須ではない。nginx は :80 の HTTP で提供。必要なら自己署名証明書で :443 も（→ [infrastructure.md](./infrastructure.md)）

## 環境

- タイムゾーンは全サービス `Asia/Tokyo`。日付は `YYYY-MM-DD` 文字列で保存・受け渡し（現行踏襲）。サーバの「今日」判定は JST
- dev は `compose.dev.yaml` でバインドマウント + ホットリロード。→ [infrastructure.md](./infrastructure.md)
- git は `git init` のみ（ローカル）。Go module パスは仮に `training-record`（ホスティング決定時に変更可）
</content>
