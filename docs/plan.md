# Web アプリ化プラン

Hermes スキル `training-tracker` を、`frontend`(Next.js) / `backend`(Go) / `nginx` の 3 サービス Web アプリに移行する。

## ゴール

- 現行スキルの記録・ルーティン・集計機能を Web UI で操作できる
- Hermes Agent は CLI ではなく backend の HTTP API 経由で記録・参照する
- `docker compose up` で一式立ち上がり、既存 `training.db` を初回に取り込む

## スコープ（memo より）

| # | 機能 | 対応ドキュメント |
|---|------|------------------|
| 1 | トレーニング記録の CRUD | [api.md](./api.md) 筋トレ/スピンセッション |
| 2 | トレーニングメニューをプリセット（ルーティン）として保持 | [api.md](./api.md) ルーティン |
| 3 | トレーニング記録の一覧表示 | frontend 一覧画面 |
| 4 | カレンダー表示 | `GET /api/calendar` + 月グリッド |
| 5 | ボリュームのグラフ表示 | `GET /api/volume` + Recharts |
| 6 | インフラ（docker-compose） | [infrastructure.md](./infrastructure.md) |
| 7 | Hermes 連携 API | [api.md](./api.md) 全体 + 認証 |

## フェーズ分割

### Phase 0 — 足場
- `git init`（ローカルのみ）+ `.gitignore`
- リポジトリレイアウト作成（`frontend/` `backend/` `nginx/` `compose.yaml` `compose.dev.yaml` `.env.example` `Makefile`）
- backend: `net/http` サーバ + `GET /api/health`（Go module パスは仮に `training-record`）
- frontend: Next.js 初期化 + トップページ
- nginx: リバースプロキシ設定（HTTP :80、LAN 提供）
- `docker compose -f compose.yaml -f compose.dev.yaml up` で 3 サービス疎通
- `Makefile` に `lint` / `test` / `build` タスク（CI は当面ローカル実行）

**完了条件**: ブラウザで nginx 経由 frontend が開き、`/api/health` が返る

### Phase 1 — backend コア（データ層 + 記録 API）
- マイグレーションランナー（`embed` SQL + `schema_migrations`）
- スキーマ `0001_init.sql`（4 テーブル + `profile` + インデックス）→ [data-model.md](./data-model.md)
- 初回ブートストラップ: `BOOTSTRAP_DB_PATH` から既存 `training.db` を取り込み
- store 層（SQL、`training_db.py` の SQL を移植）
- API: 筋トレセッション CRUD + 種目編集/並べ替え/追記、スピンセッション CRUD
- API: `GET /api/exercises`, `GET /api/sessions/last`
- API キー認証ミドルウェア
- backend のテスト（store / handler）

**完了条件**: `curl` で記録の作成・取得・更新・削除ができ、既存 36 セッションが移行済み

### Phase 2 — frontend コア（一覧 + 記録フォーム + ルーティン）
- API クライアント（`fetch` ラッパ + SWR）、SSR は `INTERNAL_API_BASE`
- 一覧画面（セッション日付降順、種目・重量・回数・notes）
- 記録の新規作成 / 編集フォーム（動的種目行、自重トグル、追記モード、種目名オートコンプリート）
- スピン記録フォーム（心拍ゾーン内訳の構造化入力 → 慣習フォーマットに整形）
- ルーティン管理画面（現在のルーティン表示・全体編集・単一種目編集・履歴）
- 「プリセットから記録」（ルーティンを新規記録の初期値に流し込み）
- API: ルーティン系（`GET/PUT /api/routine`, `PATCH /api/routine/exercises/{name}`, 履歴）

**完了条件**: UI だけで 1 日分のトレーニングを記録・修正・削除でき、ルーティンを更新できる

### Phase 3 — カレンダー表示
- API: `GET /api/calendar?month=YYYY-MM`（`notes` から `kind` 判定、日別 Volume Load）
- frontend: 月グリッド、種別バッジ（自重のみ / 自重＋FW / スピン）、日クリックで詳細
- 前月/翌月ナビ、当月ハイライト

**完了条件**: カレンダーで実施日と種別が一目でわかり、日付から記録詳細に飛べる

### Phase 4 — ボリュームグラフ + 集計
- service 層に Volume Load / TRIMP / ACWR を移植（`training_load_analysis.py`）→ [domain.md](./domain.md)
- API: `GET /api/volume`（session / week / 種目別）, `GET /api/summary`, `GET /api/summary/weekly`, `GET /api/load-report`
- API: `GET/PUT /api/profile`
- frontend: ボリューム推移グラフ（セッション別 / 週別切替）、種目別推移グラフ、ACWR ゲージ、週次サマリーカード
- プロフィール設定画面（体重・身長・推定最大心拍）

**完了条件**: ボリューム推移が可視化され、体重設定が結果に反映される

### Phase 5 — Hermes 連携
- API 全体を Hermes からの利用前提で仕上げ（エラー形・認証・CORS 不要確認）
- 新スキル `SKILL.md` を作成（CLI → HTTP に置換、`TRAINING_API_BASE` / `TRAINING_API_KEY`）
- 旧 `training_db.py` は参照用に残す（skill/ 配下は不変）
- API リファレンスを Hermes 向けに整理

**完了条件**: Hermes Agent が API 経由で記録・参照・ルーティン更新を完結できる

### Phase 6 — 故障予防アドバイス（任意）
- プログレッシブオーバーロード 10% チェック、頻度チェック、警告サイン
- デロード週リマインド
- 要チェック種目の重量増時に注意表示
- 出典: `SKILL.md` 故障予防アドバイス節、`references/`

## 依存関係

```
Phase 0 ─► Phase 1 ─► Phase 2 ─► Phase 3
                   └─► Phase 4 ─► (グラフは Phase 2 の後ならいつでも)
Phase 1 ─► Phase 5（API が揃い次第）
Phase 4 ─► Phase 6
```

## 非スコープ（今回やらない）

- マルチユーザ / 本格認証 / Web UI のログイン（単一ユーザ・LAN 内前提）
- 外部公開 / Let's Encrypt（LAN 内 HTTP。自己署名 TLS は任意で足せる程度）
- 旧 Hermes スキルとの併用・データ再同期（旧スキルは停止、初回一度きり取り込み）
- モバイルアプリ
- 種目名マスタの正規化（表記ゆれ一括修正）— 別タスク
- Postgres 移行 — SQLite で始める。必要になったら `db` サービス追加を検討
- GitHub / CI パイプライン（`git init` のみ。ホスティング決定後に検討）
</content>
