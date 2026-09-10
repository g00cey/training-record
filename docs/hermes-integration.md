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

### 依存関係

- `GET /api/advice` は backend 側で実装する（未実装）。データモデルの決定 3 点
  （痛み・違和感フィールド / デロード週の特定 / 過負荷の基準）が固まると
  レスポンスの一部項目が変わり得るが、`advice` サブコマンドは JSON をそのまま
  透過するだけなので**契約非依存**。先に追加して問題ない。

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
</content>
