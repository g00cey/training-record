#!/usr/bin/env python3
"""
Spin Bike Image Extraction Server

スピンバイクの運動結果画像を base64 で受け取り、
Vision LLM で解析して運動情報を構造化 JSON で返す HTTP エンドポイント。

使用するパッケージ: Python stdlib のみ（外部依存なし）
ポート: 8646
認証: Bearer token（SPIN_EXTRACT_API_KEY）
"""

import base64
import io
import json
import os
import signal
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Any, Optional

# ── 設定 ───────────────────────────────────────────────────────────────
PORT = int(os.environ.get("SPIN_EXTRACT_PORT", 8646))
HOST = os.environ.get("SPIN_EXTRACT_HOST", "0.0.0.0")
TIMEOUT_SECONDS = 90

# OpenCode Go の mimo-v2.5 を使用
DEFAULT_MODEL = "mimo-v2.5"
OPENCODE_GO_BASE_URL = "https://opencode.ai/zen/go/v1"

# ── 環境変数読み込み ──────────────────────────────────────────────────
def load_env_file():
    """Load env vars from /opt/data/.env if not already in os.environ."""
    env_path = "/opt/data/.env"
    if not os.path.isfile(env_path):
        return
    with open(env_path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip().strip("'\"")
            if key and key not in os.environ:
                os.environ[key] = value

load_env_file()

SPIN_EXTRACT_API_KEY = os.environ.get("SPIN_EXTRACT_API_KEY", "")

# ── Vision LLM へのプロンプト ─────────────────────────────────────────
EXTRACTION_PROMPT = """あなたはスピンバイクの運動結果画面を解析する専門家です。
この画像から運動情報を抽出し、以下の JSON スキーマに従って返してください。

## 抽出ルール
1. 読み取れない・写っていない項目は null（推測しない）
2. ゾーン名は以下の正規 5 名に正規化:
   - ウォームアップ (Zone 1, Warm-up)
   - インテンシブ (Zone 2, Intensive) 
   - 有酸素 (Zone 3, Aerobic)
   - 無酸素 (Zone 4, Anaerobic)
   - 最大酸素摂取量 (Zone 5, VO2Max, High Intensity)
3. ゾーン時間は mm:ss（60 分超は mmm:ss）
4. 心拍は整数 bpm、距離は km の小数
5. durationMinutes は整数分（h:mm 表記は換算）
6. freeNotes には心拍ゾーン内訳以外の有用な情報（消費カロリー等）を簡潔に。無ければ空文字
7. uncertainFields に読み取り確度が低い項目名を列挙。無ければ空配列
8. 日付と RPE は抽出しない（Web 側で入力・自動算出するため）

## 出力形式
コードブロックや装飾は付けず、生の JSON のみを返してください。

## JSON スキーマ
{
  "durationMinutes": integer or null,
  "avgHeartRate": integer or null,
  "maxHeartRate": integer or null,
  "distanceKm": float or null,
  "hrZones": {
    "ウォームアップ": "mm:ss" or null,
    "インテンシブ": "mm:ss" or null,
    "有酸素": "mm:ss" or null,
    "無酸素": "mm:ss" or null,
    "最大酸素摂取量": "mm:ss" or null
  },
  "freeNotes": "string",
  "uncertainFields": ["fieldName", ...]
}"""


# ── Vision LLM 呼び出し ──────────────────────────────────────────────
def call_vision_llm(image_base64: str, mime_type: str) -> dict:
    """OpenCode Go の mimo-v2.5 を使って画像解析を行う。"""
    api_key = os.environ.get("OPENCODE_GO_API_KEY", "")
    if not api_key:
        raise ValueError("OPENCODE_GO_API_KEY is not set")

    model = os.environ.get("SPIN_EXTRACT_MODEL", DEFAULT_MODEL)

    # base64 を data URI 形式に
    data_uri = f"data:{mime_type};base64,{image_base64}"

    payload = json.dumps({
        "model": model,
        "messages": [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": EXTRACTION_PROMPT},
                    {
                        "type": "image_url",
                        "image_url": {"url": data_uri}
                    }
                ]
            }
        ],
        "max_tokens": 1024,
        "temperature": 0.1
    }).encode("utf-8")

    req = urllib.request.Request(
        f"{OPENCODE_GO_BASE_URL}/chat/completions",
        data=payload,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
            # urllib のデフォルト UA (Python-urllib/x.y) は Cloudflare の
            # Bot 対策で 403 (error code: 1010) にブロックされるため明示的に上書きする
            "User-Agent": "spin-image-extraction/1.0",
            # OpenCode Go はルーティング最適化のため会話ごとに安定した
            # セッション ID を要求する。1 リクエスト = 1 会話として扱う
            "X-OpenCode-Session": str(uuid.uuid4())
        },
        method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_SECONDS) as resp:
            result = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"OpenCode Go API error {e.code}: {body}")
    except urllib.error.URLError as e:
        raise RuntimeError(f"OpenCode Go connection error: {e.reason}")

    # レスポンスからテキストを抽出
    try:
        content = result["choices"][0]["message"]["content"]
    except (KeyError, IndexError):
        raise RuntimeError(f"Unexpected API response: {json.dumps(result)[:500]}")

    # JSON をパース（コードブロック対応）
    content = content.strip()
    if content.startswith("```"):
        # コードブロックを除去
        lines = content.split("\n")
        lines = [l for l in lines if not l.strip().startswith("```")]
        content = "\n".join(lines).strip()

    try:
        extracted = json.loads(content)
    except json.JSONDecodeError as e:
        raise RuntimeError(f"Failed to parse LLM response as JSON: {e}\nContent: {content[:500]}")

    return extracted


# ── スキーマ検証 ──────────────────────────────────────────────────────
VALID_ZONE_KEYS = {"ウォームアップ", "インテンシブ", "有酸素", "無酸素", "最大酸素摂取量"}
VALID_UNCERTAIN_FIELDS = {
    "durationMinutes", "avgHeartRate", "maxHeartRate",
    "distanceKm", "hrZones", "freeNotes"
}


def validate_and_normalize(data: dict) -> dict:
    """抽出結果をスキーマに従って正規化・検証する。"""
    result = {
        "durationMinutes": None,
        "avgHeartRate": None,
        "maxHeartRate": None,
        "distanceKm": None,
        "hrZones": {},
        "freeNotes": "",
        "uncertainFields": []
    }

    # 数値フィールド
    for field in ["durationMinutes", "avgHeartRate", "maxHeartRate"]:
        val = data.get(field)
        if val is not None:
            try:
                result[field] = int(float(val))
            except (ValueError, TypeError):
                pass

    # 距離
    val = data.get("distanceKm")
    if val is not None:
        try:
            result["distanceKm"] = round(float(val), 1)
        except (ValueError, TypeError):
            pass

    # 心拍ゾーン
    hr_zones = data.get("hrZones")
    if isinstance(hr_zones, dict):
        for key, val in hr_zones.items():
            # ゾーン名の正規化
            normalized_key = normalize_zone_name(key)
            if normalized_key and val is not None:
                # 時間の形式検証 (mm:ss or mmm:ss)
                if isinstance(val, str) and validate_time_format(val):
                    result["hrZones"][normalized_key] = val

    # フリーノート
    notes = data.get("freeNotes", "")
    if isinstance(notes, str):
        result["freeNotes"] = notes

    # 不確実フィールド
    uncertain = data.get("uncertainFields", [])
    if isinstance(uncertain, list):
        result["uncertainFields"] = [
            f for f in uncertain if f in VALID_UNCERTAIN_FIELDS
        ]

    return result


def normalize_zone_name(name: str) -> Optional[str]:
    """ゾーン名を正規 5 名に正規化する。"""
    if not name:
        return None

    name_lower = name.lower().strip()

    # 日本語の正規名
    if name in VALID_ZONE_KEYS:
        return name

    # 英語表記のマッピング
    en_mapping = {
        "warm-up": "ウォームアップ",
        "warmup": "ウォームアップ",
        "zone 1": "ウォームアップ",
        "zone1": "ウォームアップ",
        "intensive": "インテンシブ",
        "zone 2": "インテンシブ",
        "zone2": "インテンシブ",
        "aerobic": "有酸素",
        "zone 3": "有酸素",
        "zone3": "有酸素",
        "anaerobic": "無酸素",
        "zone 4": "無酸素",
        "zone4": "無酸素",
        "vo2max": "最大酸素摂取量",
        "vo2 max": "最大酸素摂取量",
        "high intensity": "最大酸素摂取量",
        "zone 5": "最大酸素摂取量",
        "zone5": "最大酸素摂取量",
    }

    return en_mapping.get(name_lower)


def validate_time_format(time_str: str) -> bool:
    """mm:ss または mmm:ss の形式か検証する。"""
    if not isinstance(time_str, str):
        return False
    parts = time_str.split(":")
    if len(parts) != 2:
        return False
    try:
        minutes = int(parts[0])
        seconds = int(parts[1])
        return 0 <= minutes < 1000 and 0 <= seconds < 60
    except ValueError:
        return False


# ── HTTP ハンドラ ──────────────────────────────────────────────────────
class ExtractionHandler(BaseHTTPRequestHandler):
    """HTTP リクエストを処理するハンドラ。"""

    def log_message(self, format, *args):
        """ログフォーマットを統一。"""
        timestamp = datetime.now(timezone.utc).isoformat()
        sys.stderr.write(f"[{timestamp}] {self.address_string()} - {format % args}\n")

    def send_json(self, status: int, data: dict):
        """JSON レスポンスを送信する。"""
        response = json.dumps(data, ensure_ascii=False, indent=None)
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(response.encode("utf-8"))))
        self.end_headers()
        self.wfile.write(response.encode("utf-8"))

    def check_auth(self) -> bool:
        """Bearer token 認証を検証する。"""
        if not SPIN_EXTRACT_API_KEY:
            self.log_message("WARN: SPIN_EXTRACT_API_KEY is not configured")
            return False

        auth_header = self.headers.get("Authorization", "")
        if not auth_header.startswith("Bearer "):
            return False

        token = auth_header[7:].strip()
        return token == SPIN_EXTRACT_API_KEY

    def do_GET(self):
        """GET リクエストを処理する。"""
        if self.path == "/health":
            self.send_json(200, {
                "status": "ok",
                "version": "1.0.0",
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
        else:
            self.send_json(404, {"error": "NotFound", "message": "Endpoint not found"})

    def do_POST(self):
        """POST リクエストを処理する。"""
        # 認証チェック
        if not self.check_auth():
            self.send_json(401, {
                "error": "Unauthorized",
                "message": "Invalid or missing Bearer token"
            })
            return

        # パスチェック
        if self.path != "/extract-spin":
            self.send_json(404, {
                "error": "NotFound",
                "message": "Endpoint not found"
            })
            return

        # リクエストボディ読み込み
        content_length = int(self.headers.get("Content-Length", 0))
        if content_length == 0:
            self.send_json(400, {
                "error": "BadRequest",
                "message": "Empty request body"
            })
            return

        if content_length > 20 * 1024 * 1024:  # 20MB
            self.send_json(400, {
                "error": "BadRequest",
                "message": "Request body too large (max 20MB)"
            })
            return

        try:
            body = self.rfile.read(content_length)
            request_data = json.loads(body.decode("utf-8"))
        except json.JSONDecodeError:
            self.send_json(400, {
                "error": "BadRequest",
                "message": "Invalid JSON"
            })
            return

        # 必須フィールドチェック
        image_base64 = request_data.get("imageBase64")
        mime_type = request_data.get("mimeType")

        if not image_base64 or not mime_type:
            self.send_json(400, {
                "error": "BadRequest",
                "message": "imageBase64 and mimeType are required"
            })
            return

        # MIME タイプ検証
        if mime_type not in ("image/jpeg", "image/png", "image/webp"):
            self.send_json(400, {
                "error": "BadRequest",
                "message": f"Unsupported mimeType: {mime_type}. Use image/jpeg, image/png, or image/webp"
            })
            return

        # base64 検証
        try:
            base64.b64decode(image_base64, validate=True)
        except Exception:
            self.send_json(400, {
                "error": "BadRequest",
                "message": "Invalid base64 encoding"
            })
            return

        # Vision LLM 呼び出し
        start_time = time.time()
        try:
            self.log_message("Starting image extraction...")
            raw_result = call_vision_llm(image_base64, mime_type)
            elapsed = time.time() - start_time
            self.log_message(f"Extraction completed in {elapsed:.1f}s")
        except RuntimeError as e:
            elapsed = time.time() - start_time
            self.log_message(f"Extraction failed after {elapsed:.1f}s: {e}")
            if elapsed >= TIMEOUT_SECONDS:
                self.send_json(408, {
                    "error": "Timeout",
                    "message": "Image analysis exceeded time limit"
                })
            else:
                self.send_json(500, {
                    "error": "InternalError",
                    "message": str(e)
                })
            return
        except Exception as e:
            self.log_message(f"Unexpected error: {e}")
            self.send_json(500, {
                "error": "InternalError",
                "message": f"Unexpected error: {type(e).__name__}"
            })
            return

        # スキーマ検証・正規化
        try:
            result = validate_and_normalize(raw_result)
        except Exception as e:
            self.log_message(f"Validation error: {e}")
            self.send_json(500, {
                "error": "InternalError",
                "message": f"Result validation failed: {e}"
            })
            return

        # 成功レスポンス
        self.send_json(200, result)


# ── サーバ起動 ────────────────────────────────────────────────────────
def main():
    """メインエントリポイント。"""
    if not SPIN_EXTRACT_API_KEY:
        print("ERROR: SPIN_EXTRACT_API_KEY is not set", file=sys.stderr)
        print("Generate with: python3 -c \"import secrets; print(secrets.token_hex(32))\"", file=sys.stderr)
        sys.exit(1)

    if not os.environ.get("OPENCODE_GO_API_KEY"):
        print("ERROR: OPENCODE_GO_API_KEY is not set", file=sys.stderr)
        sys.exit(1)

    server = HTTPServer((HOST, PORT), ExtractionHandler)

    # グレースフルシャットダウン
    def signal_handler(signum, frame):
        print(f"\nReceived signal {signum}, shutting down...")
        server.shutdown()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    print(f"Spin Bike Image Extraction Server")
    print(f"  Host: {HOST}")
    print(f"  Port: {PORT}")
    print(f"  Model: {os.environ.get('SPIN_EXTRACT_MODEL', DEFAULT_MODEL)}")
    print(f"  Provider: OpenCode Go ({OPENCODE_GO_BASE_URL})")
    print(f"  Timeout: {TIMEOUT_SECONDS}s")
    print(f"  Auth: Bearer token (SPIN_EXTRACT_API_KEY)")
    print(f"\nEndpoints:")
    print(f"  GET  http://{HOST}:{PORT}/health")
    print(f"  POST http://{HOST}:{PORT}/extract-spin")
    print(f"\nStarting server...")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
        print("Server stopped.")


if __name__ == "__main__":
    main()
