# 設計ドキュメント一覧

`CLAUDE.md` の詳細版。実装の判断根拠はここに置く。

| ドキュメント | 内容 |
|--------------|------|
| [plan.md](./plan.md) | Web アプリ化の全体プランとフェーズ分割 |
| [architecture.md](./architecture.md) | コンポーネント構成・リクエストフロー・技術選定 |
| [data-model.md](./data-model.md) | DB スキーマ・既存 DB からの任意インポート |
| [api.md](./api.md) | REST API 仕様（外部クライアント共通） |
| [infrastructure.md](./infrastructure.md) | docker-compose・nginx・Dockerfile・環境変数 |
| [domain.md](./domain.md) | ドメイン用語・分析ロジック（Volume Load / ACWR / TRIMP）・落とし穴 |

## ステータス

**移行完了（2026-09-10）**。Phase 0〜6 実装・独立検証済み。
`/api/*` 経由の REST API を提供し、Web UI および任意の外部クライアント（Hermes Agent 等）が同じ backend を共有する。

## 決定事項（2026-09-08 確認済み）

| 項目 | 決定 |
|------|------|
| DB | **SQLite + named volume**（Postgres 移行は非スコープ） |
| API 認証 | **静的 API キー**（`Authorization: Bearer`）を `/api/*` 必須（`/api/health` 除く） |
| frontend CSS | **Tailwind CSS + 自前コンポーネント** |
| 配置環境 | **自宅 LAN 内の別マシン間**。外部公開なし。TLS は任意（HTTP 平文で可、自己署名 TLS もオプション） |
| Web UI のログイン認証 | **不要**（ネットワークを信頼）。ユーザ/セッション管理は作らない |
| データ移行 | **`./bootstrap/training.db` が存在すれば初回のみ取り込む**。再同期の仕組みは作らない |
| git / ホスティング | **`git init` のみ**（ローカル）。GitHub 等は未定。Go module パスは仮に `training-record`。CI は当面ローカルの `make` タスク |
| 定期バッチ実行方式（Phase 8） | **`ofelia`（Docker 向けジョブスケジューラ）を compose に追加**し label ベースで `backend` に `job-exec`（トレーニング評価・毎日03:00 JST） |
| 定期 DB バックアップ（2026-09-16 更新） | systemd timer（`db-backup.timer`）を廃止し **`ofelia` に統合**。`backend` の `job-exec` label で毎日 02:00 JST に `server -backup-daily` を実行、`./backups:/backups` の bind mount に直接書く |

軽微なデフォルト（指定あれば変更可）: パッケージマネージャ npm / Node 22・Go 1.23 / グラフ Recharts / カレンダー週初め 日曜 / UI 日本語のみ・単一ユーザ / 削除はハードデリート / LLM ファインチューニング系はスコープ外。
</content>
