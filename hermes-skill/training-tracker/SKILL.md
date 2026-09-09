---
name: training-tracker
description: Use when recording strength training or spin bike workouts, or when the user asks for training/injury prevention advice. Records via the training-record web app REST API (Japanese language support).
version: 2.0.0
author: Hermes Agent
license: MIT
metadata:
  hermes:
    tags: [fitness, training, workout, injury-prevention, http-api]
    related_skills: []
---

# Training Tracker（トレーニング記録 & 故障予防）— API 版

## Overview

ユーザーのトレーニング（筋力トレーニング + スピンバイク）を記録し、故障予防のアドバイスを行うためのスキル。

**v2.0.0 の変更**: 保存先がローカル SQLite（`~/.hermes/training-logs/training.db` を `training_db.py` で直接操作）から、
**training-record Web アプリの REST API** に移行した。データは Web アプリの backend（自宅 LAN 内）が保持する。

- **保存方式**: REST API（`TRAINING_API_BASE`）。DB は Web アプリ側が管理
- **操作用スクリプト**: `scripts/training_api.py`（旧 `training_db.py` とサブコマンド名・引数はほぼ同一）
- **ルーティン管理**: **名前付きプリセット**（既定 `自重` / `FW`）。Web アプリの UI か API で編集
- **トレーニング頻度**: スプリット構成（`references/split-routine-analysis.md`）

## セットアップ（環境変数）

スキル実行環境に以下を設定しておく（Hermes の secrets / 環境設定）:

| 変数 | 例 | 説明 |
|------|-----|------|
| `TRAINING_API_BASE` | `http://192.168.1.50/api` | Web アプリの nginx（LAN 内）+ `/api`。末尾スラッシュなし |
| `TRAINING_API_KEY` | `<64 桁の hex>` | Web アプリ backend の `API_KEY` と同一値。ユーザーから受け取る |

疎通確認:
```bash
python3 /opt/data/skills/productivity/training-tracker/scripts/training_api.py health
# → {"status":"ok","time":"...+09:00"}
```
※ `init` は不要（スキーマとデータは Web アプリ側で管理済み）。

## When to Use

- ユーザーが「トレーニング記録して」「今日のトレーニング」と言ったとき
- 「筋トレ」「スピンバイク」に関する話題、「アドバイスして」「故障予防」と言ったとき
- 「◯◯の推移」「最近の傾向」「メニュー見せて」と聞かれたとき
- ルーティン変更（重量 up、種目追加・削除等）について話したとき

## スクリプトのサブコマンド

`SCRIPT=/opt/data/skills/productivity/training-tracker/scripts/training_api.py` とする。

| コマンド | 用途 | 対応 API |
|----------|------|----------|
| `health` | 疎通確認 | `GET /api/health` |
| `record-strength --date --exercises <json> [--notes] [--append]` | 筋トレ実績を記録（既定は全置換、`--append` で追記） | `PUT /api/strength-sessions/{date}` / `POST .../exercises` |
| `record-spin --date --duration [--avg-hr --max-hr --rpe --distance --notes]` | スピンバイク記録 | `PUT /api/spin-sessions/{date}` |
| `get-history [--exercise <name>] [--limit <n>]` | 履歴 / 種目別の推移 | `GET /api/history` / `GET /api/volume?exercise=` |
| `last-session` | 直近セッション | `GET /api/sessions/last` |
| `summary [--days <n>]` | サマリー | `GET /api/summary` |
| `weekly-summary` | 週次サマリー + 評価 | `GET /api/summary/weekly` |
| `load-report` | Volume Load / ACWR / TRIMP レポート | `GET /api/load-report` |
| `get-exercise-list` | 既知の種目名一覧（表記ゆれ確認用） | `GET /api/exercises` |
| `get-routine` / `show-routine` | 現在のメニュー（全プリセット結合） | `GET /api/presets` + `GET /api/presets/{name}` |
| `get-presets` | プリセット一覧 | `GET /api/presets` |
| `show-preset --preset <name>` | 1 プリセットの内容（種目 id 付き） | `GET /api/presets/{name}` |
| `update-preset --preset <name> --exercises <json> [--date]` | プリセット全体を差し替え（履歴 +1） | `PUT /api/presets/{name}` |
| `update-exercise --preset <name> (--id <n> \| --name <name>) [--weight --reps --sets]` | プリセットの 1 種目を部分更新（`--name` で見つからなければ末尾追加） | `PATCH /api/presets/{name}/exercises/{id}` / `POST .../exercises` |
| `add-exercise --preset <name> --name <name> [--weight --reps --sets]` | プリセットに種目を追加 | `POST /api/presets/{name}/exercises` |

出力は JSON（`show-*` のみ整形テキスト）。エラー時は `{"status":"error","message":...,"http_status":...}` を返し非 0 終了。

## Workflows

### 1. 筋トレ記録

**Step 0: 実施したメニューを確認**

ユーザーに確認する:
- 「今日は自重のみですか？それとも自重＋フリーウェイトですか？」
- 「メニュー通りですか？重量・回数の変更、痛み・違和感はありましたか？」

**非同期チャット環境**: ユーザーが「自重のみ・メニュー通り・問題なし」等と明確に述べている場合、
質問を 1 行にまとめる（「自重のみ・全種目・変更なしで記録しますか？」）。

**Step 1: 現在のメニューを取得**

```bash
python3 $SCRIPT show-routine
```
番号順 = 実施順。プリセット（`自重` / `FW`）ごとに区切って表示される。

**Step 2: DB に保存**

自重のみ（全置換）:
```bash
python3 $SCRIPT record-strength \
  --date 2026-09-10 \
  --exercises '[{"name":"懸垂","reps":10},{"name":"懸垂レッグレイズ","reps":30},{"name":"ディップス","reps":30},{"name":"懸垂","reps":6},{"name":"バックエクステンション","reps":60},{"name":"ベンチレッグレイズ","reps":50},{"name":"腕立て伏せ","reps":25},{"name":"自重スクワット","reps":30},{"name":"懸垂（宽握）","reps":10}]' \
  --notes "自重のみ"
```

自重＋フリーウェイト: FW 13 種目（`weight` 付き）＋ 自重 9 種目 の全 22 種目を `--exercises` に含める。

**分割記録**（区切りごとに記録したい場合）: 1 回目は普通に、2 回目以降は `--append` を付ける。
`--append` なしは既存セッションを**全置換**する（旧 `INSERT OR REPLACE` と同じ）。`--notes` は「; 」区切りで追記される。

**重要**: `record-strength` は実績専用。プリセット（メニュー）の更新は `update-exercise` / `update-preset` で別途行う。

**Step 3: 必要ならプリセットを更新**（#4 参照）

**Step 4: 故障予防アドバイス**（下記セクション）

### 2. スピンバイク記録

| 項目 | 引数 | 必須 |
|------|------|------|
| 日付 | `--date` | ✅ `YYYY-MM-DD` |
| 時間(分) | `--duration` | ✅ |
| 平均心拍 | `--avg-hr` | 推奨 |
| 最大心拍 | `--max-hr` | 任意 |
| RPE | `--rpe` | 推奨*（1–10, Borg CR-10） |
| 距離(km) | `--distance` | 任意 |
| メモ | `--notes` | 任意 |

*心拍計がない場合は RPE で代用。`--rpe` 省略かつ `--avg-hr` `--max-hr` があれば `round(avg/max*10)` を 1–10 で自動算出。

心拍ゾーン内訳は `--notes` にこの形式で入れると `weekly-summary` が解析できる:
`心拍ゾーン内訳: ウォームアップ7:25/インテンシブ9:04/有酸素6:48/無酸素12:05/最大酸素摂取量4:42`

```bash
python3 $SCRIPT record-spin --date 2026-07-03 --duration 30 --avg-hr 148 --rpe 7
```

### 3. 情報取得

```bash
python3 $SCRIPT show-routine                       # 現在のメニュー（プリセット別）
python3 $SCRIPT get-history --exercise "ヒップストラスト"   # 種目別の重量・回数推移
python3 $SCRIPT last-session                       # 直近セッション
python3 $SCRIPT summary --days 30                  # サマリー
python3 $SCRIPT weekly-summary                     # 週次サマリー + 評価
python3 $SCRIPT load-report                        # Volume Load / ACWR / TRIMP
python3 $SCRIPT get-exercise-list                  # 既知の種目名（表記ゆれ確認）
```

**ボリューム推移**を聞かれたら `load-report`（ACWR ゾーン付き）か `get-history --exercise` を使う。
筋肥大にはボリューム増加（重量 up or 回数 up）が必要。前半 vs 後半で平均を比較して増減傾向を判断。

### 4. メニュー変更（プリセット更新）

メニューは**名前付きプリセット**（既定 `自重` / `FW`）。どのプリセットの話かを判断する
（自重種目 = `自重`、ダンベル種目 = `FW`）。

**1 種目だけ変更**:
```bash
python3 $SCRIPT show-preset --preset FW          # まず id を確認
python3 $SCRIPT update-exercise --preset FW --id 20 --weight 40 --reps 40
```
`--id` の代わりに `--name "ヒップストラスト"` でも可（同名種目が複数あるプリセットでは `--id` 必須）。
存在しない種目名を `--name` で指定すると、そのプリセットの末尾に追加される。

**プリセット全体を差し替え**:
```bash
python3 $SCRIPT update-preset --preset FW \
  --exercises '[{"name":"ヒップストラスト","weight":40,"reps":40},{"name":"ダンベルデッドリフト","weight":38,"reps":15}, ...全FW種目...]'
```
`PUT` は新しい日付でスナップショットを積む（＝変更履歴 +1）。同じ日に 2 回やると置換になる。

**種目を追加**:
```bash
python3 $SCRIPT add-exercise --preset 自重 --name "パイクプッシュアップ" --reps 15
```

**メニュー変更時のアドバイス**: `get-history --exercise <種目>` で前回実績と比較し増加が 10% 以内か確認。
全種目同時上げは避け 2–3 種目ずつローテーションで。変更後は `show-routine` でバランス確認。

## 故障予防アドバイス

### プログレッシブオーバーロード制限

- 週間総ボリューム（重量 × 回数 × セット数）の増加は **10% 以内**
- 増加 ≤10% →「問題ない範囲です」／ >15% →「少し急かもしれません。中間値から試すのも手です」
- 短期間で連続増加 →「前回からまだ間が空いていません。同じ重量で慣れてから上げましょう」
- `load-report` の ACWR も併用: 0.8–1.3 が安全、>1.5 はデロード推奨

### 要チェック種目（故障リスク高い順）

1. ショルダープレス / サイドレイズ — 肩峰下インピンジメント。重量増は慎重に
2. ディップス — 深く下ろしすぎ厳禁、上腕平行まで
3. スカルクラッシャー — 肘への負担大、コントロール優先
4. 懸垂 — 肘内側の痛みが出たら即中止、広背筋で引く
5. ダンベルデッドリフト — 腰椎注意、ヒンジ動作徹底
6. フロントラックスクワット — かかと重心、膝がつま先より前で重量過多

詳細は `references/exercise-form-guide.md`（全種目）。

### 休息・デロード

- スプリット構成（自重毎日 / FW 1 日おき）は適切 → 継続推奨
- **4–6 週ごとにデロード週**: ボリュームを 40–50% 減らす。最も効果的な故障予防
- 強い筋肉痛の翌日はスピンを軽めに（アクティブレスト）。風邪気味・睡眠不足はスキップ判断も

### 警告サイン

即中止: 鋭い/刺す痛み（特に片側）、関節の引っかかり・ロッキング、めまい・吐き気・胸痛、筋肉の「プチッ」
経過観察: 同一部位の違和感が 2 週間以上、トレ中だけの痛み、日常生活に支障、翌日に痛み増強

### 頻度チェック

`summary --days 14` で直近 2 週を確認:
- 筋トレ + スピン 合計 7 回以上/2週 → 良好
- 筋トレ 3 回未満/2週 → 頻度やや少ない、声かけ
- 間隔 24h 未満の連続 → 休息不足の警告
- 2 週間完全オフ → 再開時は重量を落としてリハビリ的にスタート

## Common Pitfalls

1. **環境変数**: `TRAINING_API_BASE` / `TRAINING_API_KEY` 未設定だと `{"status":"error"}`。まず `health` で確認。
2. **同一日付は 1 セッション**。`--append` なしの `record-strength` は既存を**全置換**。追記は `--append`。
3. **実績とプリセットは別**。`record-strength` にメニュー情報を混ぜない。プリセット更新は `update-exercise` / `update-preset`。
4. **自重種目の weight** は JSON から省略 or `null`（`0` にしない）。
5. **種目名の表記ゆれ**: `get-exercise-list` で既存名を確認して統一。
6. **同名種目の複数エントリ**（懸垂 10r / 6r）はプリセット内で別 id。`update-exercise` は `--id` で個別指定。
7. **JSON クォート**: `--exercises` はシングルクォートで囲み内部はダブルクォート。
8. **プリセットの履歴**: `update-preset`（PUT）は履歴 +1。`update-exercise`（PATCH）は最新スナップショットを in-place 更新（履歴増えない）。
9. **日付・TZ**: `YYYY-MM-DD`、`Asia/Tokyo`。サーバの「今日」は JST。

## References

- `references/exercise-form-guide.md`: 全種目のフォーム注意点・故障リスク
- `references/split-routine-analysis.md`: スプリットルーティンの分析（筋群別頻度・ボリューム戦略）

## Verification Checklist

- [ ] `health` が `{"status":"ok"}` を返す
- [ ] 筋トレ記録後、`last-session` / `get-history` で確認できる
- [ ] スピン記録後、`last-session` で確認できる
- [ ] プリセット変更後、`show-routine` に反映されている
- [ ] アドバイスは具体的な数字（重量・回数・割合・種目名）を使い、記録内容に基づいている
