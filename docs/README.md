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

Phase 0〜5 実装済み（独立検証済み）。Phase 6（故障予防アドバイス・任意）は未着手。
移行元資産は `skill/` 配下（不変）。Hermes 向け配布物は `hermes-skill/` 配下。

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

軽微なデフォルト（指定あれば変更可）: パッケージマネージャ npm / Node 22・Go 1.23 / グラフ Recharts / カレンダー週初め 月曜 / UI 日本語のみ・単一ユーザ / 削除はハードデリート / `mlops`・LLM ファインチューニング系はスコープ外。
</content>
