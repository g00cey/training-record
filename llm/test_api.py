#!/usr/bin/env python3
"""Test script for Spin Bike Image Extraction API."""
import json
import os
import sys
import urllib.request
import urllib.error

BASE_URL = "http://localhost:8647"
API_KEY = os.environ.get("SPIN_EXTRACT_API_KEY", "")


def test_health():
    """Test GET /health endpoint."""
    print("=== Test 1: Health Check ===")
    try:
        req = urllib.request.Request(f"{BASE_URL}/health")
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
            print(f"  Status: {resp.status}")
            print(f"  Response: {json.dumps(data, indent=2)}")
            assert resp.status == 200
            assert data["status"] == "ok"
            print("  ✓ PASS\n")
            return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_auth_no_token():
    """Test POST /extract-spin without Authorization header."""
    print("=== Test 2: Auth failure (no token) ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/extract-spin",
            data=json.dumps({}).encode(),
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 401
        assert data["error"] == "Unauthorized"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_auth_wrong_token():
    """Test POST /extract-spin with wrong token."""
    print("=== Test 3: Auth failure (wrong token) ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/extract-spin",
            data=json.dumps({}).encode(),
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer wrong_token_12345"
            },
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 401
        assert data["error"] == "Unauthorized"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_missing_fields():
    """Test POST /extract-spin with missing required fields."""
    print("=== Test 4: Missing required fields ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/extract-spin",
            data=json.dumps({"someField": "value"}).encode(),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {API_KEY}"
            },
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 400
        assert data["error"] == "BadRequest"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_invalid_base64():
    """Test POST /extract-spin with invalid base64."""
    print("=== Test 5: Invalid base64 ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/extract-spin",
            data=json.dumps({
                "imageBase64": "not_valid_base64!!!",
                "mimeType": "image/jpeg"
            }).encode(),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {API_KEY}"
            },
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 400
        assert data["error"] == "BadRequest"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_unsupported_mime():
    """Test POST /extract-spin with unsupported MIME type."""
    print("=== Test 6: Unsupported MIME type ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/extract-spin",
            data=json.dumps({
                "imageBase64": "dGVzdA==",
                "mimeType": "image/bmp"
            }).encode(),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {API_KEY}"
            },
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 400
        assert data["error"] == "BadRequest"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_not_found():
    """Test GET /nonexistent endpoint."""
    print("=== Test 7: Not found ===")
    try:
        req = urllib.request.Request(f"{BASE_URL}/nonexistent")
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        data = json.loads(e.read())
        print(f"  Status: {e.code}")
        print(f"  Response: {json.dumps(data, indent=2)}")
        assert e.code == 404
        assert data["error"] == "NotFound"
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


def test_method_not_allowed():
    """Test DELETE /health (wrong method)."""
    print("=== Test 8: Method not allowed ===")
    try:
        req = urllib.request.Request(
            f"{BASE_URL}/health",
            method="DELETE"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            print(f"  Status: {resp.status}")
            return False
    except urllib.error.HTTPError as e:
        print(f"  Status: {e.code}")
        # HTTPServer returns 501 for unsupported methods
        assert e.code in (405, 501)
        print("  ✓ PASS\n")
        return True
    except Exception as e:
        print(f"  ✗ FAIL: {e}\n")
        return False


if __name__ == "__main__":
    if not API_KEY:
        print("ERROR: SPIN_EXTRACT_API_KEY is not set", file=sys.stderr)
        sys.exit(1)

    print("=" * 60)
    print("Spin Bike Image Extraction API - Test Suite")
    print("=" * 60 + "\n")

    results = []
    results.append(test_health())
    results.append(test_auth_no_token())
    results.append(test_auth_wrong_token())
    results.append(test_missing_fields())
    results.append(test_invalid_base64())
    results.append(test_unsupported_mime())
    results.append(test_not_found())
    results.append(test_method_not_allowed())

    passed = sum(results)
    total = len(results)

    print("=" * 60)
    print(f"Results: {passed}/{total} tests passed")
    print("=" * 60)

    sys.exit(0 if passed == total else 1)
