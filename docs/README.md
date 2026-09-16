# 設計ドキュメント一覧

`CLAUDE.md` の詳細版。実装の判断根拠はここに置く。

| ドキュメント | 内容 |
|--------------|------|
| [plan.md](./plan.md) | Web アプリ化の全体プランとフェーズ分割 |
| [architecture.md](./architecture.md) | コンポーネント構成・リクエストフロー・技術選定 |
| [data-model.md](./data-model.md) | DB スキーマ・既存 `training.db` からの移行 |
| [api.md](./api.md) | REST API 仕様（frontend / Hermes 共通） |
| [infrastructure.md](./infrastructure.md) | docker-compose・nginx・Dockerfile・環境変数 |
| [domain.md](./domain.md) | ドメイン用語・分析ロジック（Volume Load / ACWR / TRIMP）・落とし穴 |
| [hermes-integration.md](./hermes-integration.md) | Hermes Agent 連携（Phase 5）・修正依頼文・カットオーバー手順 |

## ステータス

**移行完了（2026-09-10）**。Phase 0〜6 実装・独立検証済み。Hermes は API 版スキル（v2）に切り替え済み、
旧 `training_db.py` 系は停止。移行元資産 `skill/` は 2026-09-07 で凍結（backend の DB と一致）。
Hermes 向け配布物は `hermes-skill/`。カットオーバーの記録 → [hermes-integration.md](./hermes-integration.md)。

## 決定事項（2026-09-08 確認済み）

| 項目 | 決定 |
|------|------|
| DB | **SQLite + named volume**（Postgres 移行は非スコープ） |
| API 認証 | **静的 API キー**（`Authorization: Bearer`）を `/api/*` 必須（`/api/health` 除く） |
| frontend CSS | **Tailwind CSS + 自前コンポーネント** |
| 配置環境 | **自宅 LAN 内の別マシン間**。外部公開なし。TLS は任意（HTTP 平文で可、自己署名 TLS もオプション） |
| Web UI のログイン認証 | **不要**（ネットワークを信頼）。ユーザ/セッション管理は作らない |
| データ移行 | **旧 Hermes スキルは停止し、既存 `training.db` を初回に一度だけ取り込む**。再同期の仕組みは作らない |
| git / ホスティング | **`git init` のみ**（ローカル）。GitHub 等は未定。Go module パスは仮に `training-record`。CI は当面ローカルの `make` タスク |
| 定期バッチ実行方式（Phase 8） | **`ofelia`（Docker 向けジョブスケジューラ）を compose に追加**し label ベースで `backend` に `job-exec`。systemd timer は増やさない（`db-backup` は既存のまま） |

軽微なデフォルト（指定あれば変更可）: パッケージマネージャ npm / Node 22・Go 1.23 / グラフ Recharts / カレンダー週初め 日曜 / UI 日本語のみ・単一ユーザ / 削除はハードデリート / `mlops`・LLM ファインチューニング系はスコープ外。
</content>
