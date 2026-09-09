# hermes-skill/ — Hermes Agent 向け配布物

training-record Web アプリを Hermes Agent から使うための**新スキル**。
旧 `skill/.hermes/skills/productivity/training-tracker/`（ローカル SQLite 直接操作）の置き換え。

```
hermes-skill/training-tracker/
├── SKILL.md                      # v2.0.0。ワークフローは旧版と同じ、保存先が REST API に
├── scripts/training_api.py       # stdlib のみ。旧 training_db.py とサブコマンド互換
└── references/
    ├── exercise-form-guide.md    # 旧 skill から複製（不変）
    └── split-routine-analysis.md # 旧 skill から複製（不変）
```

## デプロイ手順（Hermes 環境側）

1. `training-tracker/` を Hermes のスキルディレクトリに配置
   （例: `/opt/data/skills/productivity/training-tracker/`）。旧 `training-tracker` は退避 or 上書き。
2. 環境変数を設定:
   - `TRAINING_API_BASE` = Web アプリ nginx の URL + `/api`（例 `http://192.168.1.50/api`）
   - `TRAINING_API_KEY` = Web アプリ backend の `API_KEY` と同一値
3. 疎通確認:
   ```bash
   python3 .../training-tracker/scripts/training_api.py health
   ```
4. `SKILL.md` の Verification Checklist を実施。

## 旧スキルとの違い

| | 旧 (v1) | 新 (v2) |
|--|--------|---------|
| 保存先 | ローカル SQLite `~/.hermes/training-logs/training.db` | Web アプリ backend（LAN 内・API 経由） |
| スクリプト | `training_db.py`（`sqlite3` 直） | `training_api.py`（`urllib` で REST） |
| 初期化 | `init` サブコマンド必須 | 不要（サーバ管理）。代わりに `health` |
| メニュー | 単一ルーティン（`routine_snapshots`） | 名前付きプリセット（`自重` / `FW` …） |
| `update-exercise` | `--name` で特定 | `--preset` + `--id`（or `--name`） |

サブコマンド名・引数は可能な限り維持。詳細は `docs/hermes-integration.md`（本リポジトリ）。
</content>
