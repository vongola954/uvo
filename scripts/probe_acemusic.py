#!/usr/bin/env python3
"""Probe AceMusic API. Usage: ACEMUSIC_API_KEY=... python scripts/probe_acemusic.py"""
from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

KEY = os.environ.get("ACEMUSIC_API_KEY", "").strip()
BASE = os.environ.get("ACEMUSIC_BASE_URL", "https://api.acemusic.ai").rstrip("/")


def call(method: str, path: str, data=None, timeout: int = 300):
    url = BASE + path
    headers = {
        "Authorization": "Bearer " + KEY,
        "Accept": "application/json",
        "User-Agent": "UVO-probe/1.0",
    }
    body = None
    if data is not None:
        body = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", "replace")
            print(f"OK {resp.status} {method} {url}", flush=True)
            print(raw[:800], flush=True)
            return resp.status, raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        print(f"HTTP {e.code} {method} {url}\n{raw[:800]}", flush=True)
        return e.code, raw
    except Exception as e:
        print(f"ERR {method} {url}: {e}", flush=True)
        return None, str(e)


def main() -> int:
    if not KEY:
        print("Set ACEMUSIC_API_KEY", file=sys.stderr)
        return 2
    call("GET", "/v1/models", timeout=30)
    call("GET", "/health", timeout=20)
    return 0


if __name__ == "__main__":
    sys.exit(main())
