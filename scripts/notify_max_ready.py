"""Send a short UVO-ready notice to a MAX chat. Usage: python scripts/notify_max_ready.py [chat_id]"""
from __future__ import annotations

import json
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path


def load_env(key: str) -> str:
    for p in (Path(__file__).resolve().parents[1] / ".env", Path(r"C:\Users\Vongola\Desktop\ScanBA\.env")):
        if not p.is_file():
            continue
        m = re.search(rf"^{re.escape(key)}=(.*)$", p.read_text(encoding="utf-8", errors="ignore"), re.M)
        if m and m.group(1).strip():
            return m.group(1).strip().strip('"').strip("'")
    return ""


def main() -> None:
    tok = load_env("MAX_BOT_TOKEN")
    if not tok:
        raise SystemExit("MAX_BOT_TOKEN missing")
    base = load_env("MAX_API_BASE") or "https://platform-api2.max.ru"
    chat_id = sys.argv[1] if len(sys.argv) > 1 else load_env("MAX_NOTIFY_CHAT_ID")
    if not chat_id:
        # fallback: last known owner dialog from ops
        chat_id = "384990975"
    text = sys.argv[2] if len(sys.argv) > 2 else "UVO готов: /health ok, max_bot=true. Студия онлайн."
    body = json.dumps({"text": text}).encode()
    url = f"{base.rstrip('/')}/messages?chat_id={chat_id}"
    req = urllib.request.Request(url, data=body, method="POST", headers={
        "Authorization": tok,
        "Content-Type": "application/json",
    })
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            print("sent", r.status, r.read()[:200])
    except urllib.error.HTTPError as e:
        print("HTTP", e.code, e.read()[:400])
        raise SystemExit(1)


if __name__ == "__main__":
    main()
