# training-record — ローカル開発 / CI 相当タスク
# ホスティング未定のため CI はこの Makefile をローカルで回す運用。

.PHONY: help up dev down build logs lint test fmt clean db_backup evaluate_training

help:
	@echo "make up      - 本番相当構成 + LLM サーバー(llm/)で起動 (docker compose up --build -d)"
	@echo "make dev     - 開発構成で起動 (compose.yaml + compose.dev.yaml, ホスト :8080)"
	@echo "make down    - 停止・コンテナ削除（LLM サーバー含む）"
	@echo "make build   - 全イメージビルド（LLM サーバー含む）"
	@echo "make logs    - 全サービスのログ追従"
	@echo "make lint    - go vet + npm run lint"
	@echo "make test    - go test ./..."
	@echo "make fmt     - gofmt"
	@echo "make clean   - down + training-data ボリューム削除（DB を初期化、LLM サーバー含む）"
	@echo "make db_backup - DB バックアップを ./backups/ に作成"
	@echo "make evaluate_training - LLM トレーニング評価バッチを手動実行（通常は ofelia が毎日自動実行）"

up:
	docker compose up --build -d
	docker compose -f llm/docker-compose.yml up --build -d

dev:
	docker compose -f compose.yaml -f compose.dev.yaml up --build

down:
	docker compose down
	docker compose -f llm/docker-compose.yml down

build:
	docker compose build
	docker compose -f llm/docker-compose.yml build

logs:
	docker compose logs -f

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint

test:
	cd backend && go test ./...

fmt:
	cd backend && gofmt -w .

clean:
	docker compose down -v
	docker compose -f llm/docker-compose.yml down -v

db_backup:
	@mkdir -p backups
	@BACKUP_FILE="backups/training_$$(date +%Y%m%d_%H%M%S).db"; \
	docker compose exec -T backend /app/server -backup /tmp/training-backup.db && \
	docker compose cp backend:/tmp/training-backup.db "$$BACKUP_FILE" && \
	echo "バックアップを作成しました: $$BACKUP_FILE"

evaluate_training:
	docker compose exec -T backend /app/server -evaluate-training
