# インフラ設計（docker-compose）

3 サービス構成: `nginx` / `frontend` / `backend`。DB は backend 内 SQLite（named volume）。

**前提（2026-09-08 確定）**: 自宅 LAN 内・外部公開なし・Web UI ログイン認証なし。
nginx は HTTP（:80）で提供する。TLS は必須ではなく、必要なら自己署名証明書で :443 も足せる（任意）。

## サービス構成

| サービス | イメージ | 内部ポート | ボリューム | 依存 |
|----------|----------|-----------|-----------|------|
| `nginx` | 自前ビルド（`nginx:1.27-alpine` ベース） | 80 → ホスト公開（443 は任意） | `./nginx/conf.d`, TLS 証明書（任意） | frontend, backend |
| `frontend` | 自前ビルド（Next.js standalone） | 3000（非公開） | なし | backend |
| `backend` | 自前ビルド（Go, scratch/distroless） | 8080（非公開） | `training-data:/data` | なし |

- ホストに晒すのは **nginx のみ**。frontend/backend は compose ネットワーク内のみ。
- `training-data` は named volume。`/data/training.db`（WAL 有効）。

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

## compose.yaml（設計ドラフト）

```yaml
name: training-record

services:
  nginx:
    build: ./nginx
    ports:
      - "80:80"
      # - "443:443"   # 自己署名 TLS を使う場合のみ有効化（任意）
    depends_on:
      frontend:
        condition: service_started
      backend:
        condition: service_healthy
    restart: unless-stopped

  frontend:
    build:
      context: ./frontend
      args:
        NEXT_PUBLIC_API_BASE: /api
    environment:
      NODE_ENV: production
      INTERNAL_API_BASE: http://backend:8080/api   # SSR フェッチ用
      TZ: Asia/Tokyo
    expose:
      - "3000"
    depends_on:
      backend:
        condition: service_healthy
    restart: unless-stopped

  backend:
    build: ./backend
    environment:
      PORT: "8080"
      DB_PATH: /data/training.db
      BOOTSTRAP_DB_PATH: /bootstrap/training.db   # 初回のみ。無ければスキップ
      API_KEY: ${API_KEY:?set API_KEY in .env}
      TZ: Asia/Tokyo
    volumes:
      - training-data:/data
      - ./skill/.hermes/home/.hermes/training-logs:/bootstrap:ro   # 既存DBの初期取り込み
    expose:
      - "8080"
    healthcheck:
      test: ["CMD", "/app/healthcheck"]           # or wget -qO- http://localhost:8080/api/health
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 20s
    restart: unless-stopped

volumes:
  training-data:
```

### compose.dev.yaml（設計ドラフト）

```yaml
services:
  frontend:
    build:
      target: dev
    command: npm run dev
    volumes:
      - ./frontend:/app
      - /app/node_modules
    environment:
      NODE_ENV: development
  backend:
    build:
      target: dev
    command: go run ./cmd/server
    volumes:
      - ./backend:/src
  nginx:
    ports:
      - "8080:80"     # dev はホスト 8080
```

起動: `docker compose -f compose.yaml -f compose.dev.yaml up --build`

## nginx（`nginx/conf.d/app.conf` ドラフト）

```nginx
upstream frontend { server frontend:3000; }
upstream backend  { server backend:8080; }

server {
    listen 80;
    server_name _;

    # LAN 内 HTTP 提供。TLS を使う場合は listen 443 ssl のサーバブロックを別途追加（任意）。
    client_max_body_size 1m;

    location /api/ {
        proxy_pass http://backend/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    location / {
        proxy_pass http://frontend;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Next.js 静的アセット
    location /_next/static/ {
        proxy_pass http://frontend;
        proxy_cache_valid 200 60m;
        add_header Cache-Control "public, max-age=3600, immutable";
    }
}
```

TLS（任意）: LAN 内なので必須ではない。使う場合は自己署名証明書を `./nginx/certs:/etc/nginx/certs:ro` でマウントし、443 の server ブロックを追加。Let's Encrypt は公開ドメインが必要なので対象外。

## Dockerfile 方針

### backend（`backend/Dockerfile`）
```dockerfile
# --- build ---
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
# modernc.org/sqlite なので CGO 不要

# --- dev ---
FROM golang:1.23 AS dev
WORKDIR /src

# --- runtime ---
FROM gcr.io/distroless/static-debian12 AS prod
COPY --from=build /out/server /app/server
COPY --from=build /src/migrations /app/migrations
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
```

### frontend（`frontend/Dockerfile`）
```dockerfile
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS dev
WORKDIR /app

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_BASE=/api
RUN npm run build          # next.config: output: 'standalone'

FROM node:22-alpine AS prod
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
| `API_KEY` | backend / frontend | API 認証キー（必須・生成する） |
| `DB_PATH` | backend | 既定 `/data/training.db` |
| `BOOTSTRAP_DB_PATH` | backend | 初回移行元。未設定/不在ならスキップ |
| `INTERNAL_API_BASE` | frontend | SSR 用 `http://backend:8080/api` |
| `NEXT_PUBLIC_API_BASE` | frontend(build) | ブラウザ用 `/api` |
| `TZ` | 全部 | `Asia/Tokyo` |

## 運用メモ

- バックアップ: `training-data` volume を `docker run --rm -v training-record_training-data:/d -v $PWD:/b alpine tar czf /b/backup.tgz -C /d .`
- 初回移行のやり直し: volume 削除 → `docker compose up`（`BOOTSTRAP_DB_PATH` から再取り込み）
- ログ: 各サービス `stdout`。集約は将来
- CI: ホスティング未定のため当面はローカルの `make` タスク（`make lint` = `go vet` + `npm run lint`、`make test` = `go test ./...`、`make build` = `docker compose build`）。GitHub 等に載せた時点で Actions 化
</content>
