"""Fetch Amvera logs with required start/end. Usage: python scripts/amvera_logs_range.py [hours] [query]"""
from __future__ import annotations

import json
import re
import sys
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path

MCP = "https://openmcp.msk0.amvera.ru/mcp"


def token() -> str:
    for p in (Path(__file__).resolve().parents[1] / ".env", Path(r"C:\Users\Vongola\Desktop\ScanBA\.env")):
        if not p.is_file():
            continue
        m = re.search(r"^AMVERA_PLATFORM_TOKEN=(.*)$", p.read_text(encoding="utf-8", errors="ignore"), re.M)
        if m and m.group(1).strip():
            return m.group(1).strip().strip('"').strip("'")
    raise SystemExit("no platform token")


def main() -> None:
    hours = float(sys.argv[1]) if len(sys.argv) > 1 else 2
    q = sys.argv[2] if len(sys.argv) > 2 else "*"
    tok = token()
    session = None
    nid = 0

    def call(method, params=None, notification=False):
        nonlocal session, nid
        body = {"jsonrpc": "2.0", "method": method}
        if not notification:
            nid += 1
            body["id"] = nid
        if params is not None:
            body["params"] = params
        h = {
            "Authorization": f"Bearer {tok}",
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
        }
        if session:
            h["Mcp-Session-Id"] = session
        req = urllib.request.Request(MCP, data=json.dumps(body).encode(), method="POST", headers=h)
        with urllib.request.urlopen(req, timeout=90) as r:
            sid = r.headers.get("Mcp-Session-Id")
            if sid:
                session = sid
            if notification:
                try:
                    r.read(64)
                except Exception:
                    pass
                return {}
            raw = r.read().decode("utf-8", "replace")
        if raw.strip().startswith("{"):
            return json.loads(raw)
        data = [l[5:].strip() for l in raw.splitlines() if l.startswith("data:")]
        return json.loads(data[-1])

    call("initialize", {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "logs", "version": "1"}})
    try:
        call("notifications/initialized", {}, notification=True)
    except Exception:
        pass

    end = datetime.now(timezone.utc)
    start = end - timedelta(hours=hours)
    res = call(
        "tools/call",
        {
            "name": "getRunLogs",
            "arguments": {
                "serviceName": "uvo",
                "query": q,
                "limit": 120,
                "start": start.strftime("%Y-%m-%dT%H:%M:%SZ"),
                "end": end.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        },
    )
    text = "\n".join(
        c.get("text", "") for c in ((res.get("result") or {}).get("content") or []) if isinstance(c, dict)
    )
    text = re.sub(r"(Authorization[=:\s]+)\S+", r"\1***", text, flags=re.I)
    text = re.sub(r"(Bearer\s+)\S+", r"\1***", text, flags=re.I)
    print(text[-10000:] if len(text) > 10000 else text)


if __name__ == "__main__":
    main()
