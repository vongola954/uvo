"""List Amvera lyrics/OpenAI env (values redacted)."""
from __future__ import annotations

import json
import re
import urllib.request
from pathlib import Path

MCP_URL = "https://openmcp.msk0.amvera.ru/mcp"
SLUG = "uvo"
KEYS = ("OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL", "OPENAI_API_BASE", "LYRICS_LLM_PROVIDER", "LYRICS_ASSIST")


def load_token() -> str:
    for p in (Path(__file__).resolve().parents[1] / ".env", Path(r"C:\Users\Vongola\Desktop\ScanBA\.env")):
        if not p.is_file():
            continue
        m = re.search(r"^AMVERA_PLATFORM_TOKEN=(.*)$", p.read_text(encoding="utf-8", errors="ignore"), re.M)
        if m:
            return m.group(1).strip().strip('"').strip("'")
    raise SystemExit("AMVERA_PLATFORM_TOKEN not found")


class MCP:
    def __init__(self, token: str) -> None:
        self.token = token
        self.session: str | None = None
        self._id = 0

    def call(self, method: str, params: dict | None = None, *, notification: bool = False) -> dict:
        body: dict = {"jsonrpc": "2.0", "method": method}
        if not notification:
            body["id"] = self._id = self._id + 1
        if params is not None:
            body["params"] = params
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
        }
        if self.session:
            headers["Mcp-Session-Id"] = self.session
        req = urllib.request.Request(MCP_URL, data=json.dumps(body).encode(), method="POST", headers=headers)
        with urllib.request.urlopen(req, timeout=60) as resp:
            sid = resp.headers.get("Mcp-Session-Id")
            if sid:
                self.session = sid
            if notification:
                try:
                    resp.read(128)
                except Exception:
                    pass
                return {}
            raw = resp.read().decode("utf-8", errors="replace")
        if raw.strip().startswith("{"):
            return json.loads(raw)
        lines = [line[5:].strip() for line in raw.splitlines() if line.startswith("data:")]
        return json.loads(lines[-1]) if lines else {}

    def tool(self, name: str, arguments: dict) -> str:
        r = self.call("tools/call", {"name": name, "arguments": arguments})
        if "error" in r:
            raise SystemExit(str(r["error"]))
        content = (r.get("result") or {}).get("content") or []
        return "\n".join(str(c.get("text", c)) if isinstance(c, dict) else str(c) for c in content)


def main() -> None:
    mcp = MCP(load_token())
    mcp.call(
        "initialize",
        {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "uvo-inspect", "version": "1"}},
    )
    mcp.call("notifications/initialized", {}, notification=True)
    for secret in (False, True):
        txt = mcp.tool("listEnvVars", {"slug": SLUG, "isSecret": secret})
        print("--- secret=%s ---" % secret)
        for line in txt.splitlines():
            if any(k in line for k in KEYS):
                # Keep name/id, redact value length only
                m = re.search(r"ID=(\d+)\s+(\S+)=(.*)$", line)
                if m:
                    val = m.group(3)
                    print("ID=%s %s len=%d empty=%s" % (m.group(1), m.group(2), len(val), val == ""))
                else:
                    print(line[:160])


if __name__ == "__main__":
    main()
