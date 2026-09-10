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
| 2 | トレーニングメニューをプリセットとして保持（自重 / FW など複数・記録フォームで加算選択） | [api.md](./api.md) プリセット、Phase 2.5 |
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

### Phase 2.5 — プリセットの複数化（追加要件）

単一ルーティンを名前付き複数プリセットに拡張。記録フォームで複数プリセットを加算的に選択できる。

- DB: `0002_add_presets.sql`（`presets` テーブル + `routine_snapshots.preset` 列 + index）。起動時 fixup `ensureRoutinePresets`（seed 自重/FW、`preset=''` 行を `weight IS NULL` で分類、`(preset,date)` で `sort_order` 振り直し）→ [data-model.md](./data-model.md)
- API: `GET /api/presets`・`GET /api/presets/{name}`・`/history`・`/{date}`、`POST /api/presets`、`PUT /api/presets:reorder`、`DELETE /api/presets/{name}`、`PUT /api/presets/{name}`、`PATCH /api/presets/{name}/exercises/{exName}`。旧 `GET /api/routine` は結合ビューとして残置、`PUT /api/routine` は 400（廃止）→ [api.md](./api.md)
- frontend:
  - `/routine`（プリセット管理）: プリセット一覧・作成・改名・削除・並べ替え、各プリセットの種目編集（`PUT`）・単一種目編集（`PATCH`）・履歴表示
  - 記録フォームの「プリセットから記録」を**複数選択**に変更。プリセットを ON にするとその種目を行として**追記**（選択順）。OFF にするとそのプリセット由来の行だけ除去（手動で追加・編集した行は残す）。各行に `sourcePreset` を持たせ、ユーザーが編集したら `sourcePreset` を外して OFF でも残す
  - 例: 「自重」ON → 自重 9 種目。続けて「FW」ON → 自重 9 + FW 13 = 22 行

**完了条件**: 記録フォームで自重・FW を個別/複合で選べ、加算・除去が仕様どおり動く。既存ルーティンが自重/FW 2 プリセットに分割移行されている

### Phase 5 — Hermes 連携 ✅ 実装済み
- 新スキル一式 `hermes-skill/training-tracker/`（`SKILL.md` v2.0.0 + `scripts/training_api.py` + `references/`）
- `training_api.py`: Python stdlib（`urllib`）のみ。旧 `training_db.py` とサブコマンド互換。
  `record-strength` / `record-spin` / `get-history` / `last-session` / `summary` / `weekly-summary` /
  `load-report` / `get-exercise-list` / `get-routine` / `show-routine` / `get-presets` / `show-preset` /
  `update-preset` / `update-exercise` / `add-exercise` / `health`
- 環境変数 `TRAINING_API_BASE` / `TRAINING_API_KEY`。`init` は廃止（サーバ管理）→ `health`
- 旧 `training_db.py` は `skill/` 配下に不変で残置
- Hermes への修正依頼文・カットオーバー手順・対応表 → [hermes-integration.md](./hermes-integration.md)

**完了条件**: Hermes Agent が API 経由で記録・参照・プリセット更新を完結できる（`training_api.py` を稼働中スタックで疎通確認済み）

### Phase 6 — 故障予防アドバイス（任意）
- backend: `GET /api/advice` — プログレッシブオーバーロード（週間VL比 + 種目別前回比）/ 頻度チェック（直近14日）/
  デロード自動判定（週間VLが直近4週平均の55%以下）/ 要チェック6種目の増量検知 / 警告サイン（notes パース）を集約。
  `service` 層に判定を集約、Web UI と Hermes `advice` サブコマンドで共用。→ [api.md](./api.md) / [domain.md](./domain.md)
- frontend: アドバイスカード（ダッシュボード / ボリューム画面。ACWR ゲージの近く）。
  ACWR ゾーン・頻度ステータス・過負荷ステータス・デロード due バナー・要チェック種目・警告サイン flag を表示
- hermes-skill: `training_api.py` に `advice` サブコマンド（薄いラッパ）。SKILL.md のアドバイス手順を `advice` 結果ベースに
- 判定の既定: 痛み=notes パース / デロード=自動判定（専用フラグなし）/ 過負荷=週間比と種目別比の両方
- 出典: `SKILL.md` 故障予防アドバイス節、`references/`
- Hermes 側への依頼文 → [hermes-integration.md](./hermes-integration.md) Phase 6

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
