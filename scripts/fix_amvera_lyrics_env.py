"""Remove broken OPENAI_API_KEY and force local-capable lyrics config on Amvera."""
from __future__ import annotations

import json
import re
import urllib.request
from pathlib import Path

MCP_URL = "https://openmcp.msk0.amvera.ru/mcp"
SLUG = "uvo"

# Prefer local fallback path in code; keep pollinations attempt but no fake key.
WANTED = {
    "OPENAI_BASE_URL": "",
    "OPENAI_MODEL": "",
    "LYRICS_LLM_PROVIDER": "auto",
    "LYRICS_ASSIST": "true",
}


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


def list_ids(mcp: MCP) -> dict[str, tuple[int, bool]]:
    found: dict[str, tuple[int, bool]] = {}
    for secret in (False, True):
        txt = mcp.tool("listEnvVars", {"slug": SLUG, "isSecret": secret})
        for vid, name in re.findall(r"ID=(\d+)\s+(\S+)=", txt):
            found[name] = (int(vid), secret)
    return found


def main() -> int:
    mcp = MCP(load_token())
    mcp.call(
        "initialize",
        {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "uvo-lyrics-fix", "version": "1"}},
    )
    mcp.call("notifications/initialized", {}, notification=True)
    ids = list_ids(mcp)

    # Delete broken/empty OpenAI key so nothing can force Pollinations paid mode.
    if "OPENAI_API_KEY" in ids:
        vid, _ = ids["OPENAI_API_KEY"]
        try:
            out = mcp.tool("deleteEnvVar", {"slug": SLUG, "id": vid})
            print("delete OPENAI_API_KEY:", out[:200].replace("\n", " "))
        except Exception as e:
            # Fallback: overwrite with placeholder that code treats as empty
            out = mcp.tool(
                "updateEnvVar",
                {
                    "slug": SLUG,
                    "id": vid,
                    "name": "OPENAI_API_KEY",
                    "value": "not-needed",
                    "isSecret": True,
                    "type": "RUN",
                },
            )
            print("update OPENAI_API_KEY placeholder:", out[:200].replace("\n", " "), "err=", e)

    for name, value in WANTED.items():
        if name in ids:
            vid, secret = ids[name]
            out = mcp.tool(
                "updateEnvVar",
                {"slug": SLUG, "id": vid, "name": name, "value": value, "isSecret": secret, "type": "RUN"},
            )
            print("update", name, ":", out[:160].replace("\n", " "))
        elif value != "":
            payload = json.dumps([{"name": name, "value": value, "isSecret": False, "type": "RUN"}], ensure_ascii=False)
            out = mcp.tool("createEnvVars", {"slug": SLUG, "envVarsJson": payload})
            print("create", name, ":", out[:160].replace("\n", " "))

    try:
        print("restart:", mcp.tool("restartProject", {"slug": SLUG})[:200].replace("\n", " "))
    except Exception as e:
        print("restart skipped:", e)
    print("done")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
