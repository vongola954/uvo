"""Fetch recent Amvera run logs related to generation (redacted)."""
from __future__ import annotations

import json
import re
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path

MCP = "https://openmcp.msk0.amvera.ru/mcp"


def platform_token() -> str:
    for p in (Path(__file__).resolve().parents[1] / ".env", Path(r"C:\Users\Vongola\Desktop\ScanBA\.env")):
        if not p.is_file():
            continue
        m = re.search(r"^AMVERA_PLATFORM_TOKEN=(.*)$", p.read_text(encoding="utf-8", errors="ignore"), re.M)
        if m and m.group(1).strip():
            return m.group(1).strip().strip('"').strip("'")
    raise SystemExit("no AMVERA_PLATFORM_TOKEN")


def main() -> None:
    tok = platform_token()
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

    call(
        "initialize",
        {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "uvo-gen-logs", "version": "1"},
        },
    )
    try:
        call("notifications/initialized", {}, notification=True)
    except Exception:
        pass

    end = datetime.now(timezone.utc)
    start = end - timedelta(hours=6)
    queries = [
        "generate OR generation OR acedata OR suno OR job OR credit OR spend OR error OR warn",
        "*",
    ]
    text = ""
    for q in queries:
        res = call(
            "tools/call",
            {
                "name": "getRunLogs",
                "arguments": {
                    "serviceName": "uvo",
                    "query": q,
                    "start": start.strftime("%Y-%m-%dT%H:%M:%SZ"),
                    "end": end.strftime("%Y-%m-%dT%H:%M:%SZ"),
                    "limit": 120,
                },
            },
        )
        text = "\n".join(
            c.get("text", "") for c in ((res.get("result") or {}).get("content") or []) if isinstance(c, dict)
        )
        if text.strip() and "validation failed" not in text.lower():
            print("QUERY", q)
            break

    # Prefer generation-related lines
    keys = (
        "generate",
        "generation",
        "acedata",
        "suno",
        "job",
        "credit",
        "spend",
        "refund",
        "poll",
        "task",
        "error",
        "warn",
        "fail",
        "401",
        "402",
        "403",
        "500",
        "provider",
        "audio",
        "track",
    )
    lines = []
    # Try parse JSON array of log objects
    try:
        # content may be pretty JSON array
        m = re.search(r"\[.*\]", text, re.S)
        if m:
            arr = json.loads(m.group(0))
            for row in arr:
                if isinstance(row, dict):
                    content = str(row.get("content") or row.get("message") or "")
                    ts = str(row.get("timestamp") or "")
                    lines.append(f"{ts} {content}")
                else:
                    lines.append(str(row))
    except Exception:
        lines = text.splitlines()

    if not lines:
        lines = text.splitlines()

    filtered = []
    for line in lines:
        low = line.lower()
        if any(k in low for k in keys):
            filtered.append(line)
    show = filtered[-80:] if filtered else lines[-80:]

    out = "\n".join(show)
    out = re.sub(r"(Authorization[=:\s]+)\S+", r"\1***", out, flags=re.I)
    out = re.sub(r"(api[_-]?key[=:\s]+)\S+", r"\1***", out, flags=re.I)
    out = re.sub(r"(token[=:\s]+)\S+", r"\1***", out, flags=re.I)
    out = re.sub(r"(password[=:\s]+)\S+", r"\1***", out, flags=re.I)
    out = re.sub(r"1a8dc77a67264adba690d7f44aeec56c", "***", out)
    out = re.sub(r"40bcd7eaa09e4d9c983bc9af1c5e8481", "***", out)
    # strip ANSI
    out = re.sub(r"\x1b\[[0-9;]*m", "", out)
    print(out if out.strip() else "(empty logs)")


if __name__ == "__main__":
    main()
