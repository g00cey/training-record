# インフラ設計（docker-compose）

3 サービス構成: `nginx` / `frontend` / `backend`。DB は backend 内 SQLite（named volume）。
Phase 8（LLM トレーニング評価の日次バッチ）追加時に、ジョブスケジューラ `ofelia` を4つ目のサービスとして足した。

**前提（2026-09-08 確定）**: 自宅 LAN 内・外部公開なし・Web UI ログイン認証なし。
nginx は HTTP（:80）で提供する。TLS は必須ではなく、必要なら自己署名証明書で :443 も足せる（任意）。

## サービス構成

| サービス | イメージ | 内部ポート | ボリューム | 依存 |
|----------|----------|-----------|-----------|------|
| `nginx` | 自前ビルド（`nginx:1.27-alpine` ベース） | 80 → ホスト公開（443 は任意） | `./nginx/conf.d`, TLS 証明書（任意） | frontend, backend |
| `frontend` | 自前ビルド（Next.js standalone） | 3000（非公開） | なし | backend |
| `backend` | 自前ビルド（`golang:1.25` → `gcr.io/distroless/static-debian12:nonroot`, CGO 無効） | 8080（非公開） | `training-data:/data` | なし |
| `ofelia` | `mcuadros/ofelia:latest` | なし（API を持たない） | `/var/run/docker.sock:ro` | backend |

- ホストに晒すのは **nginx のみ**。frontend/backend/ofelia は compose ネットワーク内のみ。
- `training-data` は named volume。`/data/training.db`（WAL 有効）。
- `ofelia` は `backend` サービスの label（`ofelia.job-exec.*`）を読み、`docker exec` で
  毎日 03:00 JST に `/app/server -evaluate-training`（Phase 8）、毎日 02:00 JST に
  `/app/server -backup-daily`（定期 DB バックアップ）を実行する。

## ファイル構成

```
training-record/
├── compose.yaml              # 本番想定のベース定義
├── compose.dev.yaml          # 開発オーバーライド（bind mount + hot reload）
├── .env.example              # API_KEY 等のサンプル
├── nginx/
│   ├── Dockerfile
│   └── conf.d/app.conf
├── frontend/
│   ├── Dockerfile
│   └── ...(Next.js)
└── backend/
    ├── Dockerfile
    ├── migrations/000x_*.sql
    └── ...(Go)
```

## compose / nginx の実ファイル

実体はリポジトリ直下:

- `compose.yaml` — 本番相当。nginx(80) / frontend / backend の 3 サービス、`training-data` volume、`./skill/.../training-logs` を `/bootstrap:ro` マウント
- `compose.dev.yaml` — dev オーバーライド（バインドマウント、nginx はホスト :8081。8080 は他コンテナと競合するため）
- `nginx/conf.d/app.conf` — `/api/` → backend（Hermes 用）、それ以外（`/bff`・`/_next`・`/`）→ frontend

以下は設計上の要点（実装で踏んだ落とし穴とその対処を含む）。

### パスの割り当て

| nginx location | 転送先 | 用途 |
|----------------|--------|------|
| `/api/` | `backend:8080` | Hermes Agent。呼び出し側が `Authorization: Bearer` を保持 |
| `/bff/spin-extract` | `frontend:3000` | スピン画像抽出（Phase 7）のみ。`client_max_body_size 12m` / `proxy_read_timeout 120s` と緩和（既定は 1m / 30s） |
| `/bff/` | `frontend:3000` | ブラウザ。Next.js Route Handler `app/bff/[...path]` がサーバ側でキーを付与し backend へ中継 |
| `/`（上記以外すべて） | `frontend:3000` | Next.js 本体・静的アセット（`/_next/...` 含む） |

- `NEXT_PUBLIC_API_BASE=/bff`（ビルド引数）、`INTERNAL_API_BASE=http://backend:8080/api`（frontend ランタイム env）、`API_KEY`（frontend ランタイム env・ブラウザには出さない）。
- ⚠️ **nginx `/api/` を frontend に向けてはいけない**。ブラウザ経路は必ず `/bff`。`/api` を frontend に回すとキー未付与で 401 になる（初回実装で発生）。
- 静的アセットの `Cache-Control` は Next.js が付ける。nginx で `add_header` を重ねない（二重ヘッダになる）。

### backend の named volume 権限（重要）

- prod イメージは distroless nonroot（UID 65532）で動作する。named volume `training-data` は初回作成時 root 所有になり、そのままでは backend が `/data/training.db` を作成できず `SQLITE_CANTOPEN` で起動失敗する。
- 対処: **backend の Dockerfile prod ステージで `/data` を UID 65532 所有で用意する**（`COPY --chown=65532:65532` で空ディレクトリを配置。空の named volume はマウント先の所有権・パーミッションをイメージから継承する）。あるいは compose に chown する init コンテナを足す。
- `make clean`（`down -v`）でボリュームを消した後の初回起動でも素で立ち上がること。

### dev の healthcheck

- `compose.yaml` の healthcheck は `["CMD", "/app/server", "-healthcheck"]`。dev イメージ（golang）には `/app/server` が無いため、`compose.dev.yaml` で backend を「一度 `go build -o /app/server` してから起動」に上書きし、同じ healthcheck を成立させる。

### TLS（任意）

LAN 内なので必須ではない。使う場合は自己署名証明書を `./nginx/certs:/etc/nginx/certs:ro` でマウントし、443 の server ブロックを追加。Let's Encrypt は公開ドメインが必要なので対象外。

## Dockerfile 方針

### backend（`backend/Dockerfile`）
```dockerfile
# --- build ---
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server   # modernc.org/sqlite なので CGO 不要
RUN mkdir -p /data && chown 65532:65532 /data            # 空 named volume に所有権を継承させる

# --- dev ---
FROM golang:1.25 AS dev
WORKDIR /src
# 起動は compose.dev.yaml が上書き: sh -c "mkdir -p /app && go build -o /app/server ./cmd/server && exec /app/server"

# --- runtime ---
FROM gcr.io/distroless/static-debian12:nonroot AS prod
COPY --from=build --chown=65532:65532 /data /data
COPY --from=build /out/server /app/server
COPY --from=build /src/migrations /app/migrations
USER 65532:65532
ENTRYPOINT ["/app/server"]
```

### frontend（`frontend/Dockerfile`）
```dockerfile
FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:24-alpine AS dev
WORKDIR /app

FROM node:24-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_BASE=/bff
RUN npm run build          # next.config: output: 'standalone'

FROM node:24-alpine AS prod
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
```

### nginx（`nginx/Dockerfile`）
```dockerfile
FROM nginx:1.27-alpine
COPY conf.d/ /etc/nginx/conf.d/
```

## 環境変数（`.env`）

| 変数 | サービス | 説明 |
|------|----------|------|
| `API_KEY` | backend / frontend(runtime) | API 認証キー（必須・生成する）。frontend では Route Handler が backend 転送時に付与 |
| `DB_PATH` | backend | 既定 `/data/training.db` |
| `BOOTSTRAP_DB_PATH` | backend | 初回移行元。未設定/不在ならスキップ |
| `HERMES_API_URL` | backend | スピン画像抽出（Phase 7）。Hermes Agent の抽出エンドポイント全体。`llm/`（`spin-extraction`）は `training-record_default` ネットワークに参加しているため `http://spin-extraction:8646/extract-spin` で到達可能。別ホストの Hermes Agent を使う場合は LAN 経由の URL（例 `http://192.168.1.50:9000/extract-spin`）。未設定なら `POST /api/spin-extract` は 503（機能無効） |
| `HERMES_API_KEY` | backend | Hermes 抽出 API の Bearer キー（Hermes Agent 側で生成・共有） |
| `LLM_EVAL_API_URL` | backend | LLM トレーニング評価（Phase 8）。`llm/` の評価エンドポイント全体（例 `http://192.168.1.50:9000/evaluate-training`）。未設定なら `server -evaluate-training` が失敗するだけ（Web UI は動作し続ける） |
| `LLM_EVAL_API_KEY` | backend | `llm/` 側の `TRAINING_EVAL_API_KEY` と同じ値 |
| `LLM_EVAL_HISTORY_WEEKS` | backend | LLM に渡す履歴の取得範囲（週）。既定 8 |
| `INTERNAL_API_BASE` | frontend(runtime) | Route Handler → backend `http://backend:8080/api` |
| `NEXT_PUBLIC_API_BASE` | frontend(build) | ブラウザ用 `/bff` |
| `TZ` | 全部 | `Asia/Tokyo` |

## 運用メモ

- バックアップ: backend の `server -backup-daily` サブコマンドが `VACUUM INTO` で WAL を畳み込んだ単一ファイル
  `training_YYYYMMDD_HHMMSS.db` を `$BACKUP_DIR`（既定 `/backups`、`./backups:/backups` でホストへ bind mount）
  に直接書く（`training.db` の単純コピーでは WAL のデータを取りこぼすため）。手動実行は `make db_backup`
  （`docker compose exec -T backend /app/server -backup-daily`）。単発で任意の宛先に書きたい場合は
  `server -backup <dest>` も残っている。volume 丸ごとの退避は
  `docker run --rm -v training-record_training-data:/d -v $PWD:/b alpine tar czf /b/backup.tgz -C /d .`
- 定期バックアップ・定期トレーニング評価（Phase 8）: systemd timer は使わず、`compose.yaml` の `ofelia` サービスが
  `backend` の label を見て毎日 02:00 JST に `/app/server -backup-daily`、毎日 03:00 JST に
  `/app/server -evaluate-training` を `docker exec` する。`docker compose up -d` するだけで有効になる
  （追加のホスト設定は不要）。確認: `docker compose logs ofelia`。手動実行・動作確認は `make db_backup` /
  `make evaluate_training`。
  `ofelia` は `/var/run/docker.sock` を読み取り専用マウントするが、Docker API 自体には読み書き制限が無いため
  実質ホストに対して強い権限を持つ。本プロジェクトは自宅 LAN 内限定・外部非公開が前提（[architecture.md](./architecture.md)）
  のためこれを許容している
- 初回移行のやり直し: volume 削除 → `docker compose up`（`BOOTSTRAP_DB_PATH` から再取り込み）
- ログ: 各サービス `stdout`。集約は将来
- CI: ホスティング未定のため当面はローカルの `make` タスク（`make lint` = `go vet` + `npm run lint`、`make test` = `go test ./...`、`make build` = `docker compose build`）。GitHub 等に載せた時点で Actions 化
</content>
