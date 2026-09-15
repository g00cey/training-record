# Hermes Agent 連携（Phase 5）

Hermes Agent の `training-tracker` スキルを、ローカル SQLite 直接操作から
**training-record Web アプリの REST API 経由**に切り替える。

- 配布物: [`hermes-skill/training-tracker/`](../hermes-skill/training-tracker/)（新 `SKILL.md` + `scripts/training_api.py` + `references/`）
- API 仕様: [api.md](./api.md)。Hermes は `/api/*`（backend 直・要 API キー）を叩く
- 前提: Web アプリが自宅 LAN 内で稼働し、Hermes の実行環境から nginx に到達できること

## 移行の考え方

- 旧 `training_db.py` は **ローカル DB を直接** 書いていた。新 `training_api.py` は **HTTP で Web アプリの DB** を更新する。
- 既存データ（`skill/.hermes/home/.hermes/training-logs/training.db`）は Web アプリ初回起動時に取り込み済み。
  **旧スキルと新スキルを併用しない**（二重記録・不整合になる）。切り替えたら旧スキルは停止する。
- サブコマンド名・引数は極力維持したので、Hermes 側のワークフロー変更は最小限。

---

## Hermes Agent に送る修正依頼（そのまま貼れる文面）

> **依頼: `training-tracker` スキルを API 版（v2）に差し替えてください**
>
> トレーニング記録の保存先を、ローカル SQLite から training-record Web アプリの REST API に移行しました。
> 以下を実施してください。
>
> **1. スキルの差し替え**
> - 新しいスキル一式を `hermes-skill/training-tracker/` として渡します（`SKILL.md` / `scripts/training_api.py` / `references/`）。
> - これを現在の `training-tracker` スキルの場所（例: `/opt/data/skills/productivity/training-tracker/`）に配置し、旧版を置き換えてください。
> - 旧 `scripts/training_db.py` と旧 `SKILL.md` は使用停止。参照用に `training_db.py` を残すのは可ですが、**呼び出さない**でください。
>
> **2. 環境変数の登録**（secrets / 環境設定に）
> - `TRAINING_API_BASE` = `http://<LAN内のホスト>/api`（末尾スラッシュなし。例 `http://192.168.1.50/api`）
> - `TRAINING_API_KEY` = `<Web アプリ backend の API_KEY と同じ値>`
>   （この値は別途安全な経路で共有します。チャット本文に平文で貼らないでください。）
>
> **3. 疎通確認**
> ```bash
> python3 <スキルパス>/scripts/training_api.py health
> # 期待: {"status":"ok","time":"...+09:00"}
> ```
>
> **4. 動作確認**（`SKILL.md` の Verification Checklist）
> - `show-routine` で現在のメニュー（プリセット `自重` / `FW`）が表示される
> - テスト日付で `record-strength` → `last-session` で確認 → その日付のセッションを Web UI 側で削除
> - `weekly-summary` / `load-report` が返る
>
> **5. 挙動の変更点（ワークフローで気をつける点）**
> - `init` は不要（サーバがスキーマ管理）。代わりに `health`。
> - メニューは**名前付きプリセット**になりました。「自重メニュー」は `--preset 自重`、「FW メニュー」は `--preset FW`。
> - 1 種目の変更は `update-exercise --preset <名> --id <n>`（`show-preset --preset <名>` で id を確認）。
>   `--name` でも可ですが、同名種目が複数あるプリセットでは `--id` を使ってください。
> - `record-strength` は既定で**その日のセッションを全置換**します（旧 `INSERT OR REPLACE` と同じ）。
>   区切って記録するときだけ 2 回目以降に `--append` を付けてください。
> - エラー時は `{"status":"error","message":...,"http_status":...}` を返して非 0 終了します。
>   `http_status: 409` は「その日付は既に記録済み」— 上書きなら `--append` なしで再実行、追記なら `--append`。
>
> **6. 完了報告**
> - 疎通・動作確認の結果と、旧スキルを停止したことを教えてください。

---

## サブコマンド対応表（旧 `training_db.py` → 新 `training_api.py`）

| 旧 | 新 | 備考 |
|----|----|----|
| `init` | （廃止）→ `health` | サーバがスキーマ管理 |
| `record-strength --date --exercises [--notes] [--append]` | 同じ | 既定=全置換、`--append`=追記 |
| `record-spin --date --duration [...]` | 同じ | RPE 自動算出も同じ |
| `get-history [--exercise] [--limit]` | 同じ | `--exercise` は種目別の推移（`/api/volume`） |
| `last-session` | 同じ | |
| `summary [--days]` | 同じ | |
| `weekly-summary` | 同じ | |
| `get-exercise-list` | 同じ | |
| （旧 `training_load_analysis.py`） | `load-report` | Volume Load / ACWR / TRIMP |
| `get-routine` / `show-routine` | 同じ（全プリセット結合表示） | プリセット別に区切って表示 |
| `update-routine --date --exercises` | `update-preset --preset <名> --exercises [--date]` | プリセット単位に |
| `update-exercise --name [--weight --reps --sets]` | `update-exercise --preset <名> (--id \| --name) [...]` | id 指定推奨 |
| （なし） | `get-presets` / `show-preset --preset <名>` / `add-exercise` | プリセット操作の追加 |

## カットオーバー手順

1. Web アプリを LAN 内で起動（`docker compose up --build -d`）。`GET /api/health` 200 を確認。
2. 既存 `training.db` が取り込まれていることを確認（Web UI の一覧 / `GET /api/summary`）。
3. Hermes に上記「修正依頼」を送付。`TRAINING_API_KEY` は安全な経路で共有。
4. Hermes 側で `health` と動作確認。
5. 旧スキル（`training_db.py`）を停止。以降ローカル `training.db` は更新されない。
6. 以後、記録は Web UI と Hermes のどちらから行っても同じ backend DB に入る。

## 移行の残作業チェックリスト（運用）— **完了（2026-09-10）**

Phase 0〜6 の Web アプリ実装は完了・検証済み。カットオーバーも下記のとおり完了。

### 必須
- [x] **Hermes 疎通の最終確認**: Hermes 実行環境から `training_api.py health` 200 確認済み（`TRAINING_API_BASE=http://<LAN内のホスト>/api`）
- [x] **旧スキルの停止確認**: Hermes 側で旧 `training_db.py` 系は一切呼ばれていない。旧ローカル `training.db` は凍結（2026-09-07 時点、backend の DB がこれと一致）
- [x] **未記録データの投入**: 未記録データなし。既に Web UI からデータ投入済み
- [x] **常時起動**: `docker compose up -d`（`restart: unless-stopped`）で常時稼働・ホスト再起動後も自動復帰
- [ ] **エンドツーエンド確認（任意・未実施）**: Hermes からの記録が Web UI に出ることを一度通しで見る、は未確認。Hermes 疎通・旧スキル停止・Web UI 投入が済んでいるため実運用は開始可能

### 判断済み（スコープ外）
- **API キーのローテーション**: 実施しない（現行キー維持）
- **定期バックアップ**: 本スコープ外
- **`.env` の控え**: 当面は取らない
- **TLS**: なし（LAN 内 HTTP のまま）
- **feature/multi-preset ブランチ**: 削除済み
- **Phase 6 の Hermes 側 `advice` 組み込み**: 実施済み

## ロールバック

- Web アプリ稼働後に Hermes 経由で入れた記録は Web アプリの DB（`training-data` volume）にのみ存在する。
  旧ローカル `training.db` には戻らない。ロールバックが必要なら、Web アプリ DB から
  該当期間をエクスポートして旧 `training.db` に取り込む手作業が要る（通常は不要）。
- スキル自体のロールバックは旧 `training-tracker`（v1）に戻すだけ。

## 既知の注意点

- `TRAINING_API_KEY` は静的キー。LAN 内前提の軽い防御。ブラウザ／外部には出さない。
- Hermes の実行環境から LAN の nginx に到達できること（別ネットワークなら VPN / ポートフォワード等が必要）。
- `training_api.py` は Python 標準ライブラリのみ（`urllib`）。追加パッケージ不要。

---

## Phase 6（故障予防アドバイス）— Hermes 側への依頼

Phase 6 では backend に `GET /api/advice`（判定の集約）を追加し、Hermes 側は
`training_api.py` に `advice` サブコマンドを足して、その結果 + `references/` から
自然文アドバイスを構成する。**Hermes が実装するのはサブコマンドとワークフローのみ**。

### 状況（2026-09-10）

- `GET /api/advice` は backend 実装済み・独立検証済み（稼働中スタックで応答）。
- Hermes 側の `advice` サブコマンド追加・`SKILL.md` のアドバイス手順組み込みも**実施済み**。
- 判定の既定（痛み=notes パース / デロード=週間VLが直近4週平均の55%以下の自動判定 / 過負荷=週間比+種目別前回比）は
  [api.md](./api.md)「故障予防アドバイス（Phase 6）」に確定記載。
- 以下の「依頼文」は経緯の記録（再送・再構築が必要な場合の参照用）。

### Hermes Agent に送る依頼（そのまま貼れる文面）

> **依頼: `training-tracker` スキルに `advice` サブコマンドを追加してください（Phase 6）**
>
> 故障予防アドバイス用に、backend へ `GET /api/advice` エンドポイントを追加します
> （判定ロジックはサーバ側。Web UI と共用）。Hermes 側では次を実施してください。
>
> **1. `scripts/training_api.py` にサブコマンドを追加**
> `load-report` と同じ薄いラッパです。3 か所に追記:
> ```python
> # ハンドラ（cmd_load_report の隣）
> def cmd_advice(a):
>     _print(_request("GET", "/advice"))
>
> # パーサ登録（sub.add_parser("load-report") の隣）
> sub.add_parser("advice")
>
> # handlers dict（"load-report": cmd_load_report, の隣）
> "advice": cmd_advice,
> ```
> 使い方: `python3 <スキルパス>/scripts/training_api.py advice`
>
> **2. `GET /api/advice` のレスポンス形（backend が返すもの）**
> ```json
> {
>   "asOf": "2026-09-10",
>   "acwr": { "value": 1.05, "zone": "safe", "acute7d": 180000, "chronicWeekly": 172000 },
>   "frequency": { "windowDays": 14, "strengthSessions": 8, "spinSessions": 0, "total": 8,
>                  "maxConsecutiveWithin24h": 2,
>                  "status": "good", "message": "..." },   // good | low | rest_needed | long_off
>   "progressiveOverload": { "weeklyVolumeChangePct": 6.2, "status": "ok",   // ok | caution | warning
>       "exercises": [ { "name": "ヒップストラスト", "prevWeight": 38, "latestWeight": 40,
>                        "changePct": 5.3, "status": "ok", "lastIncreasedOn": "2026-09-07" } ] },
>   "deload": { "lastDeloadDate": "2026-08-05", "weeksSince": 5, "due": true, "message": "..." },
>   "watchExercises": [ { "name": "ショルダープレス", "reason": "weight_increased",
>                         "from": 22, "to": 25, "on": "2026-09-09", "formGuideAnchor": "ショルダープレス" } ],
>   "warningSigns": { "flaggedSessions": [ { "date": "2026-09-06", "matched": ["違和感"], "notes": "..." } ],
>                     "stopNow": [ "..." ], "monitor": [ "..." ] }
> }
> ```
>
> **3. ワークフローへの組み込み（`SKILL.md`）**
> - 「筋トレ記録」の最後（Step 4 / 故障予防アドバイス）を、`advice` の結果を根拠に
>   **具体的な数値・種目名を挙げて**話す手順に差し替え。
> - `progressiveOverload.status` が `caution`/`warning`、`acwr.zone` が `caution`/`warning`、
>   `deload.due` が true、`frequency.status` が `rest_needed`/`long_off` のときは明確に警告。
> - `watchExercises` に該当があれば `references/exercise-form-guide.md` の該当種目の行を引用。
> - `warningSigns.flaggedSessions` があれば「即中止すべき」「経過観察」のリストを提示。
> - 非同期チャットなので簡潔に。「ただ気をつけて」ではなく数字で。
>
> **4. 動作確認**
> ```bash
> python3 <スキルパス>/scripts/training_api.py advice
> # → 上記の JSON。http_status 404 なら backend 側が未デプロイ
> ```
>
> **5. 完了報告**: サブコマンド追加と、`SKILL.md` のワークフロー更新箇所を教えてください。
>
> ※ `GET /api/advice` の一部項目（`deload` の判定方法、`warningSigns` の拾い方など）は
> データモデルの決定次第で変わる可能性があります。決まり次第この文面を更新します。

---

## Phase 7（スピンバイク画像登録）— Hermes 側への依頼

スピンバイクの運動結果画面（アプリのスクリーンショット等）を Web UI からアップロードして
運動情報（時間・距離・平均/最大心拍・心拍ゾーン別の時間内訳）を自動入力する機能。
Web アプリ側（backend `POST /api/spin-extract` → Hermes Agent の抽出 API）は **実装済み**。
Hermes 側には **画像を受け取って構造化 JSON を返す HTTP エンドポイント** を用意してもらう。

- 状況（2026-09-10）: 7A（Web 側・モック Hermes 検証済み）完了。7B（Hermes 側）は下記依頼文の送付待ち。

### データの流れ（Phase 7 のみ逆向きの呼び出し）

```
ブラウザ ── POST /bff/spin-extract (multipart 画像) ──► nginx ──► Next.js (/bff) ──► backend /api/spin-extract
                                                                                          │ base64 JSON + Bearer
                                                                                          ▼
                                                                                Hermes Agent 抽出 API（本依頼）
                                                                                          │ 構造化 JSON
frontend: フォーム自動反映 → ユーザー確認・修正 → POST /api/spin-sessions で保存（既存フロー）
```

抽出と保存を分離しているため、Hermes 側は**保存しない（抽出のみ）**。
日付はフォーム入力、RPE は保存時の自動算出（avg/max から）に任せるため、いずれも抽出対象外。

### Hermes Agent に送る依頼（そのまま貼れる文面）

> **依頼: スピンバイクの運動結果画像から運動情報を抽出する HTTP API を用意してください（Phase 7）**
>
> training-record Web アプリに、スピンバイクの運動結果画面（スクリーンショット等）を
> アップロードすると運動情報をフォームに自動入力する機能を追加しました。
> 画像からの情報抽出を Hermes Agent に依頼したいので、次の HTTP エンドポイントを用意してください。
>
> **1. エンドポイント**
> - `POST <Hermes 側で決めた URL>`（例: `http://<Hermesのホスト>:<ポート>/extract-spin`）。
>   パス・ポートは Hermes 側の都合で決めて構いません。決めたら URL を教えてください。
> - 認証: `Authorization: Bearer <静的キー>`。キーは Hermes 側で生成して安全な経路で共有してください
>   （この Web アプリの `API_KEY` とは別の値で構いません）。
> - 応答は **90 秒以内**（Web 側の経路が 120 秒で切れるため）。長い場合は最悪でも破棄してエラー応答。
>
> **2. リクエスト（Web アプリ backend が送るもの）**
> ```json
> POST /extract-spin
> Authorization: Bearer <キー>
> Content-Type: application/json
>
> { "imageBase64": "<JPEG/PNG/WebP 画像の base64>", "mimeType": "image/jpeg" }
> ```
>
> **3. レスポンス（Hermes が返すもの・厳密な契約）**
> ```json
> {
>   "durationMinutes": 52,
>   "avgHeartRate": 130,
>   "maxHeartRate": 156,
>   "distanceKm": 20.3,
>   "hrZones": {
>     "ウォームアップ": "8:16",
>     "インテンシブ": "10:42",
>     "有酸素": "6:48",
>     "無酸素": "12:05",
>     "最大酸素摂取量(高負荷)": "4:42"
>   },
>   "freeNotes": "消費カロリー 480kcal",
>   "uncertainFields": ["distanceKm"]
> }
> ```
>
> 抽出ルール:
> - **読み取れない・写っていない項目は `null`**（推測しない）。`hrZones` の各ゾーンも同様に `null`
> - ゾーン名（`hrZones` のキー）は上記 **5 つの正規名** に正規化して返す
>   （アプリの表記ゆれ・英語表記・Zone 1–5 等はこの 5 名に割り当て。該当しないものは省略）
> - ゾーンの時間は `mm:ss`（60 分超は `mmm:ss`）。`durationMinutes` は整数分（`h:mm` 表記は換算）
> - 心拍は整数 bpm、距離は km の小数
> - `freeNotes` には心拍ゾーン内訳以外の有用な情報（消費カロリー等）を簡潔に。無ければ空文字
> - `uncertainFields` に読み取り確度が低い項目名（`durationMinutes` / `avgHeartRate` / `maxHeartRate` /
>   `distanceKm` / `hrZones` / `freeNotes` のいずれか）を列挙。無ければ `[]`
> - **日付と RPE は抽出しない**（Web 側で入力・自動算出するため）
> - レスポンスは **JSON のみ**（``` コードブロック等の装飾を付けない。Web 側でも防御的にパースするが契約は素の JSON）
> - 実装方式（既存スキルのワークフローでも専用ハンドラでも）は問いません。
>   要は「画像を投げたら上記 JSON が返る HTTP エンドポイント」です。
>
> **4. 動作確認（Hermes 側）**
> ```bash
> curl -s -X POST <URL> -H "Authorization: Bearer <キー>" \
>   -H "Content-Type: application/json" \
>   -d '{"imageBase64":"<適当な画像のbase64>","mimeType":"image/png"}'
> # → 上記の形の JSON
> ```
>
> **5. 完了報告**: URL とキー、実装方式の概要、上記契約から変えた点があればその内容を教えてください。
>   受け取ったら Web アプリ側の `.env` に設定して実画像で E2E 確認します。

### 7B の手順（Hermes 側 API が用意されたら）

1. `.env` に `HERMES_API_URL`（報告された URL）と `HERMES_API_KEY` を設定
2. `docker compose up -d --build backend` で再作成（起動ログに `hermes image extraction enabled` と出る）
3. Web UI → スピン新規登録 → 画像アップロード → 「画像を解析」→ 自動入力 → 修正 → 保存
4. 一覧・カレンダーに反映されること、`last-session`（Hermes 側）でも見えることを確認

### 既知の注意点（Phase 7）

- Hermes 抽出 API は画像しか受け取らず保存しない。保存は Web アプリの `POST /api/spin-sessions`
- 画像は Web アプリに保存されない（抽出のために一時的に扱うのみ）
- `HERMES_API_URL` 未設定でも Web アプリは起動する（`/api/spin-extract` は 503、UI は手入力のまま）
</content>
