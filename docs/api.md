# REST API 仕様

backend が公開する API。**frontend と Hermes Agent の共通インターフェース**。
現行 CLI（`training_db.py`）のサブコマンドを REST に再編したもの。

## 共通事項

- ベース URL:
  - Hermes Agent: `/api`（nginx → backend 直）。コンテナ内は `http://backend:8080/api`
  - ブラウザ（frontend）: `/bff`（nginx → Next.js Route Handler → backend）。frontend のクライアントコードは `/bff/*` だけを叩き、キー付与はサーバ側で行う
  - エンドポイントのパス（`/strength-sessions` 等）は両者共通。以下の記載は `/api` で統一表記
- 認証: `Authorization: Bearer <API_KEY>`（`/api/health` を除く全エンドポイント必須）。欠落/不一致は `401`
  - frontend はブラウザから直接叩かず、Next.js サーバ側がキーを付与して backend を呼ぶ。Hermes Agent は自分でキーを保持（LAN 越し）
  - CORS 応答は返さない（同一オリジン / サーバ間のみ想定）
- Content-Type: `application/json; charset=utf-8`
- 日付: `YYYY-MM-DD`（文字列）。タイムゾーンは `Asia/Tokyo`
- 種目名キーは全エンドポイントで **`name`** に統一（DB の `exercise_name` は API 境界で変換）
- 自重種目の `weight` は `null`（`0` ではない）

### レスポンス形

成功はリソースを直接返す（`200` / `201` / `204`）。エラーは:

```json
{ "error": { "code": "not_found", "message": "strength session 2026-09-10 not found" } }
```

`code` 例: `bad_request` / `unauthorized` / `not_found` / `conflict` / `unprocessable` / `internal`

### CLI → API 対応表

| `training_db.py` | API |
|---|---|
| `record-strength` | `POST /api/strength-sessions` / `PUT /api/strength-sessions/{date}` |
| `record-strength --append` | `POST /api/strength-sessions/{date}/exercises` |
| `record-spin` | `POST /api/spin-sessions` / `PUT /api/spin-sessions/{date}` |
| `get-history` | `GET /api/strength-sessions` / `GET /api/history` |
| `last-session` | `GET /api/sessions/last` |
| `summary` | `GET /api/summary` |
| `weekly-summary` | `GET /api/summary/weekly` |
| `get-routine` / `show-routine` | `GET /api/presets` / `GET /api/presets/{name}`（旧 `GET /api/routine` は結合ビューとして残置） |
| `update-routine` | `PUT /api/presets/{name}` |
| `update-exercise` | `PATCH /api/presets/{name}/exercises/{exName}` |
| `get-exercise-list` | `GET /api/exercises` |
| （新規） | `GET /api/calendar` / `GET /api/volume` / `GET /api/load-report` / `GET|PUT /api/profile` |
| （新規・Phase 6） | `GET /api/advice`（skill: `advice`） |

---

## エンドポイント

### ヘルスチェック
```
GET /api/health → 200 { "status": "ok", "time": "2026-09-08T12:00:00+09:00" }
```

### 筋トレセッション

#### 一覧
```
GET /api/strength-sessions?from=2026-08-01&to=2026-08-31&limit=30&offset=0
→ 200 { "items": [ StrengthSession... ], "total": 36 }
```
`from`/`to` 省略時は全期間、`limit` 既定 30、日付降順。

#### 取得
```
GET /api/strength-sessions/2026-09-07 → 200 StrengthSession | 404
```

#### 作成（新規のみ。既存日付は 409）
```
POST /api/strength-sessions
{
  "date": "2026-09-08",
  "notes": "自重＋フリーウェイト",
  "exercises": [
    { "name": "ヒップストラスト", "weight": 40, "reps": 40, "sets": 1, "notes": "" },
    { "name": "懸垂", "weight": null, "reps": 10 }
  ]
}
→ 201 StrengthSession | 409 (conflict, 既存日付)
```
`sort_order` は配列順で自動採番。

#### 置き換え（upsert・全種目を差し替え = 現行 `record-strength` 既定挙動）
```
PUT /api/strength-sessions/2026-09-08
{ "notes": "...", "exercises": [ ... ] }
→ 200 StrengthSession (新規なら 201)
```

#### メモのみ更新
```
PATCH /api/strength-sessions/2026-09-08
{ "notes": "自重＋フリーウェイト; 肩に違和感" }
→ 200 StrengthSession
```

#### 種目の追記（現行 `--append`。notes は "; " 連結）
```
POST /api/strength-sessions/2026-09-08/exercises
{ "exercises": [ { "name": "腕立て伏せ", "reps": 30 } ], "notes": "自重のみ" }
→ 200 StrengthSession
```

#### 種目の個別編集 / 削除 / 並べ替え
```
PUT    /api/strength-sessions/2026-09-08/exercises/{exerciseId}
       { "name": "...", "weight": 42, "reps": 40, "sets": 1, "notes": "" } → 200
DELETE /api/strength-sessions/2026-09-08/exercises/{exerciseId} → 204
PUT    /api/strength-sessions/2026-09-08/exercises:reorder
       { "orderedIds": [12, 10, 11, ...] } → 200 StrengthSession
```

#### セッション削除
```
DELETE /api/strength-sessions/2026-09-08 → 204   (exercises は CASCADE)
```

### スピンバイクセッション

```
GET    /api/spin-sessions?from=&to=&limit=  → 200 { "items": [...], "total": n }
GET    /api/spin-sessions/2026-07-22        → 200 SpinSession | 404
POST   /api/spin-sessions                   → 201 | 409
PUT    /api/spin-sessions/2026-07-22        → 200/201
PATCH  /api/spin-sessions/2026-07-22        → 200   (部分更新)
DELETE /api/spin-sessions/2026-07-22        → 204
```
`POST`/`PUT` body:
```json
{ "date": "2026-07-22", "duration_minutes": 52, "avg_heart_rate": 130,
  "max_heart_rate": 156, "rpe": null, "distance_km": null,
  "notes": "心拍ゾーン内訳: ウォームアップ8:16/インテンシブ10:42/..." }
```
`rpe` が `null` かつ `avg_heart_rate` と `max_heart_rate` があれば `round(avg/max*10)` を 1–10 にクランプして保存（現行踏襲）。

### プリセット（名前付きメニュー）

自重 / FW などを名前付きプリセットとして複数保持する。記録フォームは複数プリセットを選んで**加算的に**種目を補完する（→ frontend）。

```
GET /api/presets
→ 200 { "presets": [
    { "name": "自重", "sortOrder": 0, "exerciseCount": 9,  "latestDate": "2026-07-30" },
    { "name": "FW",   "sortOrder": 1, "exerciseCount": 13, "latestDate": "2026-07-30" }
  ] }

GET /api/presets/自重
→ 200 { "name": "自重", "date": "2026-07-30",
        "exercises": [ { "id": 4, "name": "懸垂", "weight": null, "reps": 10, "sets": 1 }, ... ] }
   | 404 (プリセットが無い / スナップショットが無い)

GET /api/presets/自重/history      → 200 { "snapshots": [ { "date": "2026-07-30", "exerciseCount": 9 }, { "date": "2026-07-03", "exerciseCount": 9 } ] }
GET /api/presets/自重/2026-07-03   → 200 (指定スナップショット) | 404
```
- URL のプリセット名・種目名は要 URL エンコード。

#### プリセット作成 / 並べ替え / 削除
```
POST /api/presets            { "name": "コンディショニング", "sortOrder": 2 }  → 201 { "name": ..., "sortOrder": 2 }
                             既存名 → 409 conflict
PUT  /api/presets:reorder    { "order": ["FW", "自重", "コンディショニング"] }   → 200 { "presets": [...] }
                             order は既存プリセット全部をちょうど 1 回ずつ含むこと。
                             未知の名前 / 重複 / 個数不一致 → 400 bad_request
DELETE /api/presets/コンディショニング  → 204  (そのプリセットの routine_snapshots も削除) / 未知の名前は 404
```

#### プリセット全体を更新（新スナップショットを積む＝履歴 +1。日付は今日）
```
PUT /api/presets/自重
{ "exercises": [ { "name": "懸垂", "weight": null, "reps": 10, "sets": 1 }, ... ] }
→ 200 { "name": "自重", "date": "<today>", "exercises": [...] }
```
- プリセットが未登録なら**作成**（末尾の `sortOrder`）してからスナップショットを積む。
- 任意で `{ "date": "2026-09-08", "exercises": [...] }` と明示可。
- スナップショットは `(preset, date)` 単位。**同じ日に 2 回 PUT すると履歴は増えず内容が置換される**。履歴を増やしたいときは別日付を指定する。

#### 単一種目の部分更新（現行 `update-exercise` 相当）
```
PATCH /api/presets/自重/exercises/{exerciseId}
{ "name": "懸垂（狭）", "reps": 12 }   // name / weight / reps / sets のうち送ったものだけ変更
→ 200 { "action": "updated", "exerciseId": 4, "exercise": "懸垂（狭）", "preset": "自重", "presetDate": "2026-07-30" }
```
- `exerciseId` は `routine_snapshots.id`。同名種目が複数ある場合でも個別に指定可能
- 存在しない `exerciseId` → `404`
- プリセット未登録（`presets` に無い名前）→ `404`
- 登録済みだがスナップショット未作成（`POST /api/presets` 直後など）→ `404`（スナップショットが存在しない場合は追加しない）
- **最新スナップショットを in-place 更新**（履歴は増やさない）

#### 種目の追加（新規）
```
POST /api/presets/自重/exercises
{ "name": "懸垂（狭）", "weight": null, "reps": 10, "sets": 1 }
→ 201 { "action": "added", "exerciseId": 42, "exercise": "懸垂（狭）", "preset": "自重", "presetDate": "2026-07-30" }
```
- プリセット未登録（`presets` に無い名前）→ `404`
- 登録済みだがスナップショット未作成 → 今日付でスナップショットを作成し追加
- 既存種目と同名でも追加可能（同名種目の複数登録を許容）

### ルーティン（旧・互換読み取り）

`GET /api/routine` は全プリセットの最新を `sortOrder` 順に結合した読み取りビュー。Hermes 連携（Phase 5）まで残す。

```
GET /api/routine
→ 200 { "date": "<全プリセット最新日の max>",
        "presets": ["自重", "FW"],
        "exercises": [ { "id": 4, "name": "懸垂", "weight": null, "reps": 10, "sets": 1, "sortOrder": 0 }, ... ] }
        // exercises は presets 配列の順（= sortOrder 順）→ 各プリセット内 sort_order 順で連結
        // ※ 現状は行に preset 名を含めない。どのプリセット由来かは presets 配列の順と件数から判断するか、
        //    プリセット別に見たい場合は GET /api/presets/{name} を使う
   | 200 { "date": null, "presets": [], "exercises": [] }   // presets はあるがスナップショット未作成
   | 404 (`presets` テーブルが空)

GET /api/routine/history     → 200 { "snapshots": [ { "date": "2026-07-30", "exerciseCount": 22 }, ... ] }  // 全プリセット合算
GET /api/routine/2026-07-03  → 200 (その日の全プリセット結合) | 404

PUT   /api/routine                    → 400 bad_request  { "message": "use PUT /api/presets/{name}" }（廃止）
PATCH /api/routine/exercises/{name}
                                        → 全プリセットの最新スナップショットを横断検索して該当種目を in-place 更新。
                                        複数プリセットに同名種目があれば sortOrder 最小のプリセットを対象。
                                        同名種目を個別に更新したい場合は PATCH /api/presets/{name}/exercises/{exerciseId} を使う。
                                        どこにも無ければ 404（この経路では追加しない）。
                                        → 200 { "action": "updated", "exercise": ..., "preset": ..., "presetDate": ... }
```

### 種目マスタ
```
GET /api/exercises
→ 200 { "exercises": ["ディップス", "バックエクステンション", "ヒップストラスト", ...] }
```
`exercises` + `routine_snapshots` から `DISTINCT` した名前のソート済みリスト（現行 `get-exercise-list`）。

### プロフィール
```
GET /api/profile → 200 { "bodyweightKg": 86.0, "heightCm": 170.0, "maxHrEst": 180 }
PUT /api/profile { "bodyweightKg": 85.0 } → 200 (部分更新可)
```

---

## 集計・ビュー系

### カレンダー表示用
```
GET /api/calendar?month=2026-09
→ 200 {
  "month": "2026-09",
  "days": [
    { "date": "2026-09-05", "strength": { "kind": "fw_only", "exerciseCount": 13, "volumeLoad": 41000 },
      "spin": null },
    { "date": "2026-09-06", "strength": { "kind": "bodyweight_only", "exerciseCount": 9, "volumeLoad": 22000 },
      "spin": null },
    { "date": "2026-09-07", "strength": { "kind": "bodyweight_and_fw", "exerciseCount": 22, "volumeLoad": 63000 },
      "spin": { "durationMinutes": 30, "rpe": 7 } }
  ]
}
```
`kind` は `notes` から判定: `bodyweight_only` / `fw_only` / `bodyweight_and_fw` / `other`。実施のない日は `days` に含めない。

### ボリューム推移グラフ用
```
GET /api/volume?granularity=session&from=2026-07-01&to=2026-09-30
→ 200 { "granularity": "session",
        "points": [ { "date": "2026-07-03", "volumeLoad": 38500, "exerciseCount": 19 }, ... ] }

GET /api/volume?granularity=week
→ 200 { "granularity": "week",
        "points": [ { "weekStart": "2026-07-28", "volumeLoad": 210000, "sessionCount": 4 }, ... ] }

GET /api/volume?exercise=ヒップストラスト
→ 200 { "exercise": "ヒップストラスト",
        "points": [ { "date": "2026-07-03", "weight": 38, "reps": 40, "sets": 1, "volumeLoad": 1520 }, ... ] }
```
Volume Load = `Σ weight × reps × sets`。自重種目は `profile.bodyweightKg` を weight とみなす（`BODYWEIGHT_EXERCISES` 相当の判定 → [domain.md](./domain.md)）。

### サマリー（現行 `summary`）
```
GET /api/summary?days=30
→ 200 { "periodDays": 30, "since": "2026-08-09",
        "strengthSessionsInPeriod": 20, "spinSessionsInPeriod": 0,
        "totalStrengthSessions": 36, "totalSpinSessions": 5,
        "latestStrengthDate": "2026-09-07", "latestSpinDate": "2026-07-22",
        "latestRoutineDate": "2026-07-30" }
```

### 週次サマリー + 評価（現行 `weekly-summary`）
```
GET /api/summary/weekly
→ 200 { "period": "2026-09-01 ~ 2026-09-08",
        "summary": { "totalTrainingSessions": 6, "spinSessions": {...}, "strengthSessions": {...} },
        "evaluation": ["✅ トレーニング頻度：良好（5-6回/週）", ...],
        "advice": ["故障予防のため、2日に1回の頻度を維持", ...] }
```

### 負荷レポート ACWR / TRIMP（現行 `training_load_analysis.py`）
```
GET /api/load-report
→ 200 { "asOf": "2026-09-08",
        "profile": { "bodyweightKg": 86, "heightCm": 170, "bmi": 29.8 },
        "acute7d": { "strengthSessions": 4, "spinSessions": 0, "volumeLoad": 180000, "spinTrimp": 0, "total": 180000, "volumeLoadPerBw": 2093 },
        "chronic28d": { "strengthSessions": 16, "weeklyVolumeLoad": 175000 },
        "acwr": 1.03,
        "zone": "safe",              // safe | caution | warning | low | slightly_low | no_data
        "latestSessionBreakdown": [ { "name": "ヒップストラスト", "volumeLoad": 1600, "isBodyweight": false }, ... ],
        "recommendations": ["現状のペースを維持してください", "4-6週間に1回、デロード週を..."] }
```

### 直近セッション（現行 `last-session`）
```
GET /api/sessions/last
→ 200 { "strength": StrengthSession | null, "spin": SpinSession | null }
```

### 故障予防アドバイス（Phase 6）

`SKILL.md`「故障予防アドバイス」節と `docs/domain.md` の判定ルールを集約。Web UI のアドバイスカード・Hermes の `advice` サブコマンド共用。

```
GET /api/advice
→ 200
{
  "asOf": "2026-09-10",
  "acwr": { "value": 1.05, "zone": "safe", "acute7d": 180000, "chronicWeekly": 172000 },
  "frequency": {
    "windowDays": 14,
    "strengthSessions": 8, "spinSessions": 0, "total": 8,
    "maxConsecutiveWithin24h": 2,
    "status": "good",          // good(合計≥7) | low(筋トレ<3) | rest_needed(24h未満連続あり) | long_off(直近14日ゼロ)
    "message": "直近2週で8回。良好"
  },
  "progressiveOverload": {
    "weeklyVolumeChangePct": 6.2,                                // 直近7日 vs その前7日の総 Volume Load
    "status": "ok",           // ok(≤10%) | caution(>10%) | warning(>15% または同一種目の短期連続増)
    "exercises": [
      { "name": "ヒップストラスト", "prevWeight": 38, "latestWeight": 40,
        "changePct": 5.3, "status": "ok", "lastIncreasedOn": "2026-09-07" }
    ]     // 直近セッションで重量が前回実績より増えた種目のみ
  },
  "deload": {
    "lastDeloadDate": "2026-08-05" | null,   // 自動判定: 週間VLが直近4週平均の55%以下の週
    "weeksSince": 5 | null,
    "due": true,             // weeksSince が null または ≥5
    "message": "5週間デロードなし。今週ボリュームを40–50%落とすことを検討"
  },
  "watchExercises": [        // 固定6種目（ショルダープレス/サイドレイズ/ディップス/スカルクラッシャー/懸垂/ダンベルデッドリフト/フロントラックスクワット）
    { "name": "ショルダープレス", "reason": "weight_increased",
      "from": 22, "to": 25, "on": "2026-09-09", "formGuideAnchor": "ショルダープレス" }
  ],
  "warningSigns": {
    "flaggedSessions": [ { "date": "2026-09-06", "matched": ["違和感"], "notes": "…肩に違和感…" } ],
    "stopNow": [ "鋭い/刺す痛み（特に片側）", "関節の引っかかり・ロッキング感", "めまい・吐き気・胸の痛み", "筋肉の『プチッ』（肉離れの疑い）" ],
    "monitor": [ "同じ部位の違和感が2週間以上", "トレーニング中だけ出る痛み", "日常生活に支障", "翌日に痛みが強くなる" ]
  }
}
```

**判定の既定（`docs/domain.md` 参照）**:
- 痛み・違和感は `strength_sessions.notes` をキーワード（`痛`, `違和感`, `しびれ`, `ロッキング`, `肉離れ`, `張り` 等）でパース。直近 28 日を対象
- デロード週は自動判定（週間 Volume Load が直近 4 週平均の 55% 以下）。専用フラグは持たない
- プログレッシブオーバーロードは「週間総ボリューム比」と「種目別の前回実績比」の両方を返す

Hermes 側はこの JSON ＋ `references/exercise-form-guide.md` から自然文アドバイスを構成する（→ [hermes-integration.md](./hermes-integration.md) Phase 6）。

---

## スキーマ（レスポンスオブジェクト）

### StrengthSession
```json
{
  "date": "2026-09-07",
  "notes": "自重＋フリーウェイト",
  "createdAt": "2026-09-07T15:04:07+09:00",
  "updatedAt": "2026-09-07T15:04:07+09:00",
  "exercises": [
    { "id": 540, "name": "ヒップストラスト", "weight": 40, "reps": 40, "sets": 1, "notes": "", "sortOrder": 0 }
  ]
}
```

### SpinSession
```json
{
  "date": "2026-07-22", "durationMinutes": 52, "avgHeartRate": 130, "maxHeartRate": 156,
  "rpe": 8, "distanceKm": null, "notes": "心拍ゾーン内訳: ...",
  "createdAt": "2026-07-22T14:36:08+09:00", "updatedAt": "2026-07-22T14:36:08+09:00"
}
```

JSON キーは **camelCase**。DB は snake_case（境界で変換）。リクエストボディは camelCase / snake_case どちらも受理（スピン・プロフィール）。未知フィールドは無視。

## 実装で確定した挙動（2026-09-08）

初回実装で docs との差分を吸収した点。テスト・Hermes 連携はこちらに合わせる。

- `GET /api/history?limit=` … CLI 対応表にあるがエンドポイント一覧から漏れていた。`{ "strengthSessions": [...], "spinSessions": [...] }`（`limit` 既定 10、日付降順）で実装
- `POST /api/strength-sessions/{date}/exercises` … 対象日付が未登録なら**セッションを新規作成**して追記（現行 `--append` フォールバック準拠）。`200`、`notes` は `"; "` 連結
- `PUT /api/strength-sessions/{date}/exercises:reorder` … `orderedIds` にそのセッションに属さない id → `422 unprocessable`
- `GET /api/volume?exercise=` の `weight` … 記録された生の重量（自重種目は `null`）。`volumeLoad` には自重体重の代入を適用
- カレンダー `kind` 判定 … `自重なし` / `自重無し` を除去してから `自重` 部分一致を見る（「（自重なし）」を `bodyweight_and_fw` に誤分類しない）
- エラーコード … 構造・フォーマット不正 → `bad_request`(400)、意味的なフィールド不正（`reps` 欠落・`rpe` が 1–10 外）→ `unprocessable`(422)。不明ルート → JSON の `not_found`
- タイムスタンプ … アプリ書き込み分は RFC3339 `+09:00`。レガシー `YYYY-MM-DD HH:MM:SS`(UTC) は読み取り時に変換
- 丸め … `volumeLoad` と load-report 合計は整数、`acwr` 2 桁、`spinTrimp` / `volumeLoadPerBw` / `bmi` 1 桁

## Hermes Agent 側の移行（Phase 5・実装済み）

旧スキルの `python3 training_db.py <cmd>`（ローカル SQLite 直）を、この API を叩く新スキルに置き換えた。

- 新スキル一式: [`hermes-skill/training-tracker/`](../hermes-skill/training-tracker/)
  （`SKILL.md` v2.0.0 + `scripts/training_api.py` + `references/`）
- `training_api.py` は Python 標準ライブラリ（`urllib`）のみ。旧 `training_db.py` とサブコマンド名・引数を極力維持
- Hermes 環境の環境変数: `TRAINING_API_BASE`（例 `http://192.168.1.50/api`）/ `TRAINING_API_KEY`（backend の `API_KEY` と同値）
- **Hermes Agent への具体的な修正依頼文・カットオーバー手順・サブコマンド対応表 → [hermes-integration.md](./hermes-integration.md)**
</content>
