"""Set CREDITS_UNLIMITED=true on Amvera project via MCP."""
from __future__ import annotations

import json
import re
import sys
import urllib.request
from pathlib import Path

MCP_URL = "https://openmcp.msk0.amvera.ru/mcp"
SLUG = "uvo"
ENV_KEY = "CREDITS_UNLIMITED"
ENV_VALUE = "true"


def _read_env_value(path: Path, key: str) -> str:
    if not path.is_file():
        return ""
    text = path.read_text(encoding="utf-8", errors="ignore")
    m = re.search(rf"^{re.escape(key)}=(.*)$", text, re.M)
    if not m:
        return ""
    return m.group(1).strip().strip('"').strip("'")


def load_platform_token() -> str:
    for p in (
        Path(__file__).resolve().parents[1] / ".env",
        Path(r"C:\Users\Vongola\Desktop\ScanBA\.env"),
    ):
        v = _read_env_value(p, "AMVERA_PLATFORM_TOKEN")
        if v:
            return v
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
            raise SystemExit(f"MCP error: {r['error']}")
        content = (r.get("result") or {}).get("content") or []
        return "\n".join(
            str(c.get("text", c)) if isinstance(c, dict) else str(c) for c in content
        )


def main() -> int:
    mcp = MCP(load_platform_token())
    mcp.call(
        "initialize",
        {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "uvo-credits-unlimited", "version": "1.0"},
        },
    )
    mcp.call("notifications/initialized", {}, notification=True)

    existing_id = None
    for secret_flag in (False, True):
        txt = mcp.tool("listEnvVars", {"slug": SLUG, "isSecret": secret_flag})
        for vid, name in re.findall(r"ID=(\d+)\s+(\S+)=", txt):
            if name == ENV_KEY:
                existing_id = int(vid)
                print(f"found {ENV_KEY} id={existing_id} isSecret={secret_flag}")

    if existing_id is not None:
        out = mcp.tool(
            "updateEnvVar",
            {
                "slug": SLUG,
                "id": existing_id,
                "name": ENV_KEY,
                "value": ENV_VALUE,
                "isSecret": False,
                "type": "RUN",
            },
        )
        print("updateEnvVar:", out[:500])
    else:
        payload = json.dumps(
            [{"name": ENV_KEY, "value": ENV_VALUE, "isSecret": False, "type": "RUN"}],
            ensure_ascii=False,
        )
        out = mcp.tool("createEnvVars", {"slug": SLUG, "envVarsJson": payload})
        print("createEnvVars:", out[:500])
    print("CREDITS_UNLIMITED=true set on Amvera (restart/redeploy needed)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
