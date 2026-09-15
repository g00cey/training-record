---
name: spin-image-extraction
version: 1.0.0
description: Extract spin bike data from images via HTTP endpoint.
metadata:
  hermes:
    tags: [fitness, spin-bike, image-extraction, http-api, vision-llm]
    related_skills: [training-tracker]
    requires_tools: [terminal, web_extract, write_file]
    requires_toolsets: []
    requires_plugins: []
---

# Spin Bike Image Extraction（スピンバイク画像解析）

## Overview

スピンバイクの運動結果画面（スクリーンショット等）を base64 エンコードして送ると、
Vision LLM で画像解析し、運動情報を構造化 JSON で返す HTTP エンドポイント。

- **サーバ**: Python stdlib のみ（外部パッケージ不使用）
- **抽出エンジン**: OpenCode Go mimo-v2.5（Hermes のデフォルトモデル）
- **ポート**: 8646（Hermes ホスト `172.16.10.1`）
- **認証**: Bearer token（`SPIN_EXTRACT_API_KEY`）

## エンドポイント

| メソッド | パス | 説明 |
|----------|------|------|
| `POST` | `/extract-spin` | 画像から運動情報を抽出 |
| `GET` | `/health` | ヘルスチェック（認証不要） |

## セットアップ

### 1. 環境変数

Hermes の `.env` に以下を設定（既に設定済みの場合あり）:

```
SPIN_EXTRACT_API_KEY=<64桁のhex>  # Hermes 側で生成・管理
OPENCODE_GO_API_KEY=<設定済み>      # OpenCode Go Vision LLM 用
```

### 2. サーバ起動

```bash
python3 /opt/data/skills/fitness/spin-image-extraction/server.py &
```

または s6 サービスとして永続化:

```bash
# s6 スキャンディレクトリにリンク
ln -s /opt/data/skills/fitness/spin-image-extraction/s6/spin-extraction /etc/s6/sv/spin-extraction
s6-svc -u /run/s6/services/spin-extraction
```

### 3. 疎通確認

```bash
curl -s http://172.16.10.1:8646/health
curl -s -X POST http://172.16.10.1:8646/extract-spin \
  -H "Authorization: Bearer $SPIN_EXTRACT_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"imageBase64":"<test_image>","mimeType":"image/jpeg"}'
```

## 使い方（Web アプリ側）

Web アプリの `.env` に設定:

```
HERMES_API_URL=http://172.16.10.1:8646/extract-spin
HERMES_API_KEY=<SPIN_EXTRACT_API_KEY と同じ値>
```

フロー:
1. ユーザーがスピンバイクの結果画像をアップロード
2. Web アプリが画像を base64 エンコード
3. `POST /extract-spin` に送信
4. 受信した JSON をフォームに自動入力

## レスポンススキーマ

```json
{
  "durationMinutes": 52,
  "avgHeartRate": 130,
  "maxHeartRate": 156,
  "distanceKm": 20.3,
  "hrZones": {
    "ウォームアップ": "8:16",
    "インテンシブ": "10:42",
    "有酸素": "6:48",
    "無酸素": "12:05",
    "最大酸素摂取量": "4:42"
  },
  "freeNotes": "消費カロリー 480kcal",
  "uncertainFields": ["distanceKm"]
}
```

## 抽出ルール

1. **読み取れない項目は null**（推測しない）
2. **ゾーン名は正規 5 名に正規化**:
   - ウォームアップ / インテンシブ / 有酸素 / 無酸素 / 最大酸素摂取量
   - 英語表記（Zone 1-5, Warm-up, Intensive, Aerobic, Anaerobic, VO2Max）もこの 5 名に割り当て
3. **ゾーン時間は mm:ss**（60 分超は mmm:ss）
4. **心拍は整数 bpm**、距離は km の小数
5. **freeNotes**: 心拍ゾーン内訳以外の有用な情報（消費カロリー等）。無ければ空文字
6. **uncertainFields**: 読み取り確度が低い項目名を列挙。無ければ空配列
7. **日付と RPE は抽出しない**（Web 側で入力・自動算出するため）
8. **レスポンスは JSON のみ**（コードブロック等の装飾なし）

## 既存スキルとの連携

抽出結果を training-tracker で記録する場合:

```bash
python3 /opt/data/skills/productivity/training-tracker/scripts/training_api.py \
  record-spin --date $(date +%Y-%m-%d) \
  --duration <durationMinutes> \
  --avg-hr <avgHeartRate> \
  --max-hr <maxHeartRate> \
  --distance <distanceKm>
```

心拍ゾーン内訳は `--notes` に形式を合わせて渡す:

```
心拍ゾーン内訳: ウォームアップ8:16/インテンシブ10:42/有酸素6:48/無酸素12:05/最大酸素摂取量4:42
```

## Common Pitfalls

1. **base64 サイズ制限**: 画像が大きいと OpenRouter のペイロード制限に抵触。2048x2048 以下にリサイズ推奨
2. **タイムアウト**: 90 秒制限。大きい画像・遅いモデルは超過の可能性あり
3. **日本語ゾーン名**: 画像内の表記が英語の場合でも正規 5 名に正規化して返す
4. **認証キー**: Web アプリ側の `HERMES_API_KEY` と Hermes 側の `SPIN_EXTRACT_API_KEY` は同一値

## References

- `references/spin-image-extraction-openapi.yaml`: OpenAPI 3.0 スペック
- 親ディレクトリの `training-tracker`: 記録用スキル

## Verification Checklist

- [ ] `/health` が `200 OK` を返す
- [ ] 認証なしで `/extract-spin` を呼ぶと `401` が返る
- [ ] テスト画像で `/extract-spin` を呼び、スキーマに合った JSON が返る
- [ ] 読み取れない項目が `null` になる
- [ ] ゾーン名が正規 5 名に正規化される
- [ ] 90 秒以内にレスポンスが返る