#!/usr/bin/env python3
"""
Training Tracker - HTTP client for the training-record web app.

旧 training_db.py（ローカル SQLite 直接操作）の置き換え。
サブコマンド名・引数は旧スクリプトにできるだけ合わせてあるので、
SKILL.md のワークフローはコマンドのパスと環境変数を変えるだけで移行できる。

環境変数:
  TRAINING_API_BASE   例: http://192.168.1.50/api   （末尾スラッシュなし）
  TRAINING_API_KEY    backend の API_KEY と同じ静的キー

Usage:
  training_api.py health
  training_api.py record-strength --date YYYY-MM-DD --exercises <json> [--notes <text>] [--append]
  training_api.py record-spin --date YYYY-MM-DD --duration <min> [options]
  training_api.py get-history [--exercise <name>] [--limit <n>]
  training_api.py last-session
  training_api.py summary [--days <n>]
  training_api.py weekly-summary
  training_api.py load-report
  training_api.py get-exercise-list
  training_api.py get-routine
  training_api.py show-routine
  training_api.py get-presets
  training_api.py show-preset --preset <name>
  training_api.py update-preset --preset <name> --exercises <json> [--date YYYY-MM-DD]
  training_api.py update-exercise --preset <name> (--id <n> | --name <name>) [--weight <kg>] [--reps <n>] [--sets <n>]
  training_api.py add-exercise --preset <name> --name <name> [--weight <kg>] [--reps <n>] [--sets <n>]
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("TRAINING_API_BASE", "").rstrip("/")
KEY = os.environ.get("TRAINING_API_KEY", "")
TIMEOUT = 15


def _fail(msg, http_status=None):
    out = {"status": "error", "message": msg}
    if http_status is not None:
        out["http_status"] = http_status
    print(json.dumps(out, ensure_ascii=False, indent=2))
    sys.exit(1)


def _request(method, path, body=None, query=None, allow_404=False):
    if not BASE:
        _fail("TRAINING_API_BASE が未設定です")
    url = BASE + path
    if query:
        q = {k: v for k, v in query.items() if v is not None}
        if q:
            url += "?" + urllib.parse.urlencode(q)
    data = None
    headers = {"Accept": "application/json"}
    if path != "/health" and KEY:
        headers["Authorization"] = "Bearer " + KEY
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json; charset=utf-8"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            raw = resp.read().decode("utf-8")
            if resp.status == 204 or not raw:
                return {"_status": resp.status}
            return json.loads(raw)
    except urllib.error.HTTPError as e:
        if e.code == 404 and allow_404:
            return None
        raw = e.read().decode("utf-8", "replace")
        try:
            parsed = json.loads(raw)
            msg = parsed.get("error", {}).get("message", raw)
        except Exception:
            msg = raw or e.reason
        _fail(f"{method} {path} -> {e.code}: {msg}", http_status=e.code)
    except urllib.error.URLError as e:
        _fail(f"{method} {path} 接続失敗: {e.reason}")


def _quote(seg):
    return urllib.parse.quote(seg, safe="")


def _print(obj):
    print(json.dumps(obj, ensure_ascii=False, indent=2))


# ---------------------------------------------------------------------------
# handlers
# ---------------------------------------------------------------------------

def cmd_health(a):
    _print(_request("GET", "/health"))


def cmd_record_strength(a):
    exercises = json.loads(a.exercises)
    if a.append:
        body = {"exercises": exercises}
        if a.notes:
            body["notes"] = a.notes
        res = _request("POST", f"/strength-sessions/{_quote(a.date)}/exercises", body)
    else:
        # 旧 record-strength の既定は INSERT OR REPLACE = 全置換
        body = {"notes": a.notes or "", "exercises": exercises}
        res = _request("PUT", f"/strength-sessions/{_quote(a.date)}", body)
    _print({"status": "ok", "date": a.date, "result": res})


def cmd_record_spin(a):
    body = {
        "date": a.date,
        "duration_minutes": a.duration,
        "avg_heart_rate": a.avg_hr,
        "max_heart_rate": a.max_hr,
        "rpe": a.rpe,
        "distance_km": a.distance,
        "notes": a.notes or "",
    }
    res = _request("PUT", f"/spin-sessions/{_quote(a.date)}", body)
    _print({"status": "ok", "date": a.date, "result": res})


def cmd_get_history(a):
    if a.exercise:
        # 種目別の推移は volume エンドポイントが持っている
        res = _request("GET", "/volume", query={"exercise": a.exercise})
        _print({"status": "ok", "exercise": a.exercise, "records": res.get("points", res)})
    else:
        res = _request("GET", "/history", query={"limit": a.limit})
        _print(res)


def cmd_last_session(a):
    _print(_request("GET", "/sessions/last"))


def cmd_summary(a):
    _print(_request("GET", "/summary", query={"days": a.days}))


def cmd_weekly_summary(a):
    _print(_request("GET", "/summary/weekly"))


def cmd_load_report(a):
    _print(_request("GET", "/load-report"))


def cmd_get_exercise_list(a):
    _print(_request("GET", "/exercises"))


def cmd_get_routine(a):
    # 全プリセット結合ビュー。出力キーは旧 get-routine に合わせて exercise_name も添える
    res = _request("GET", "/routine")
    exs = []
    for e in res.get("exercises", []):
        d = dict(e)
        d.setdefault("exercise_name", d.get("name"))
        exs.append(d)
    _print({"status": "ok", "date": res.get("date"),
            "presets": res.get("presets", []), "exercises": exs})


def _fmt_ex(i, e):
    w = f"{e['weight']}kg" if e.get("weight") is not None else "自重"
    sets = e.get("sets", 1) or 1
    sets_str = f" × {sets}セット" if sets > 1 else ""
    return f"{i}. {e.get('name') or e.get('exercise_name')} : {w} × {e['reps']}{sets_str}"


def cmd_show_routine(a):
    presets = _request("GET", "/presets").get("presets", [])
    if not presets:
        print("【現在の筋トレメニュー】\n（プリセットが登録されていません）")
        return
    total = 0
    print("【現在の筋トレメニュー】")
    for p in presets:
        name = p["name"]
        snap = _request("GET", f"/presets/{_quote(name)}", allow_404=True)
        if snap is None:
            print(f"\n[{name}] （スナップショット未作成）")
            continue
        exs = snap.get("exercises", [])
        print(f"\n[{name}]（{snap.get('date')} 時点・{len(exs)} 種目）")
        print("-" * 44)
        for i, e in enumerate(exs, 1):
            print("  " + _fmt_ex(i, e))
        total += len(exs)
    print("-" * 44)
    print(f"全 {total} 種目 / プリセット {len(presets)} 個")


def cmd_get_presets(a):
    _print(_request("GET", "/presets"))


def cmd_show_preset(a):
    res = _request("GET", f"/presets/{_quote(a.preset)}")
    print(f"【プリセット: {res['name']}】（{res.get('date')} 時点）")
    print("-" * 44)
    for i, e in enumerate(res.get("exercises", []), 1):
        print("  " + _fmt_ex(i, e) + f"   (id={e.get('id')})")
    print("-" * 44)
    print(f"全 {len(res.get('exercises', []))} 種目")


def cmd_update_preset(a):
    body = {"exercises": json.loads(a.exercises)}
    if a.date:
        body["date"] = a.date
    _print(_request("PUT", f"/presets/{_quote(a.preset)}", body))


def _resolve_exercise_id(preset, name):
    res = _request("GET", f"/presets/{_quote(preset)}", allow_404=True)
    if res is None:
        return None
    hits = [e for e in res.get("exercises", []) if (e.get("name") == name)]
    if len(hits) == 1:
        return hits[0]["id"]
    if len(hits) == 0:
        return None
    ids = ", ".join(str(e["id"]) for e in hits)
    _fail(f"プリセット {preset!r} に {name!r} が複数あります（id: {ids}）。--id で指定してください")


def cmd_update_exercise(a):
    changes = {}
    if a.weight is not None:
        changes["weight"] = a.weight
    if a.reps is not None:
        changes["reps"] = a.reps
    if a.sets is not None:
        changes["sets"] = a.sets
    if a.name and a.id:
        changes["name"] = a.name  # rename は id 指定時のみ
    if not changes:
        _fail("変更項目（--weight / --reps / --sets）を指定してください")

    ex_id = a.id
    if ex_id is None:
        if not a.name:
            _fail("--id か --name のどちらかが必要です")
        ex_id = _resolve_exercise_id(a.preset, a.name)
        if ex_id is None:
            # 旧挙動: 存在しない種目名は末尾に追加
            body = {"name": a.name, **{k: v for k, v in changes.items() if k != "name"}}
            _print(_request("POST", f"/presets/{_quote(a.preset)}/exercises", body))
            return
    _print(_request("PATCH", f"/presets/{_quote(a.preset)}/exercises/{ex_id}", changes))


def cmd_add_exercise(a):
    body = {"name": a.name}
    if a.weight is not None:
        body["weight"] = a.weight
    if a.reps is not None:
        body["reps"] = a.reps
    if a.sets is not None:
        body["sets"] = a.sets
    _print(_request("POST", f"/presets/{_quote(a.preset)}/exercises", body))


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    p = argparse.ArgumentParser(description="Training Tracker HTTP client")
    sub = p.add_subparsers(dest="command", required=True)

    sub.add_parser("health")

    rs = sub.add_parser("record-strength")
    rs.add_argument("--date", required=True)
    rs.add_argument("--exercises", required=True,
                    help='JSON: [{"name":"...","weight":38,"reps":40,"sets":1,"notes":""}]')
    rs.add_argument("--notes", default="")
    rs.add_argument("--append", action="store_true",
                    help="既存セッションに種目を追記（省略時は全置換）")

    rsp = sub.add_parser("record-spin")
    rsp.add_argument("--date", required=True)
    rsp.add_argument("--duration", type=int, required=True)
    rsp.add_argument("--avg-hr", dest="avg_hr", type=int, default=None)
    rsp.add_argument("--max-hr", dest="max_hr", type=int, default=None)
    rsp.add_argument("--rpe", type=int, default=None)
    rsp.add_argument("--distance", type=float, default=None)
    rsp.add_argument("--notes", default="")

    gh = sub.add_parser("get-history")
    gh.add_argument("--exercise", default=None)
    gh.add_argument("--limit", type=int, default=10)

    sub.add_parser("last-session")

    sm = sub.add_parser("summary")
    sm.add_argument("--days", type=int, default=30)

    sub.add_parser("weekly-summary")
    sub.add_parser("load-report")
    sub.add_parser("get-exercise-list")
    sub.add_parser("get-routine")
    sub.add_parser("show-routine")
    sub.add_parser("get-presets")

    shp = sub.add_parser("show-preset")
    shp.add_argument("--preset", required=True)

    up = sub.add_parser("update-preset")
    up.add_argument("--preset", required=True)
    up.add_argument("--exercises", required=True,
                    help='JSON: [{"name":"...","weight":38,"reps":40,"sets":1}]')
    up.add_argument("--date", default=None)

    ue = sub.add_parser("update-exercise")
    ue.add_argument("--preset", required=True)
    ue.add_argument("--id", type=int, default=None, help="routine_snapshots.id（同名種目の個別指定）")
    ue.add_argument("--name", default=None, help="種目名で特定（1件のときのみ。無ければ末尾に追加）")
    ue.add_argument("--weight", type=float, default=None)
    ue.add_argument("--reps", type=int, default=None)
    ue.add_argument("--sets", type=int, default=None)

    ae = sub.add_parser("add-exercise")
    ae.add_argument("--preset", required=True)
    ae.add_argument("--name", required=True)
    ae.add_argument("--weight", type=float, default=None)
    ae.add_argument("--reps", type=int, default=None)
    ae.add_argument("--sets", type=int, default=None)

    a = p.parse_args()
    handlers = {
        "health": cmd_health,
        "record-strength": cmd_record_strength,
        "record-spin": cmd_record_spin,
        "get-history": cmd_get_history,
        "last-session": cmd_last_session,
        "summary": cmd_summary,
        "weekly-summary": cmd_weekly_summary,
        "load-report": cmd_load_report,
        "get-exercise-list": cmd_get_exercise_list,
        "get-routine": cmd_get_routine,
        "show-routine": cmd_show_routine,
        "get-presets": cmd_get_presets,
        "show-preset": cmd_show_preset,
        "update-preset": cmd_update_preset,
        "update-exercise": cmd_update_exercise,
        "add-exercise": cmd_add_exercise,
    }
    try:
        handlers[a.command](a)
    except json.JSONDecodeError as e:
        _fail(f"--exercises の JSON が不正です: {e}")


if __name__ == "__main__":
    main()
