"""Set MAX_BOT_TOKEN on Amvera project via MCP. Never prints secret values."""
from __future__ import annotations

import json
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

MCP_URL = "https://openmcp.msk0.amvera.ru/mcp"
SLUG = "uvo"
ENV_KEY = "MAX_BOT_TOKEN"
TOKEN_FILES = [
    Path(__file__).resolve().parents[1] / ".env",
    Path(r"C:\Users\Vongola\Desktop\ScanBA\.env"),
]


def _read_env_value(path: Path, key: str) -> str:
    if not path.is_file():
        return ""
    text = path.read_text(encoding="utf-8", errors="ignore")
    m = re.search(rf"^{re.escape(key)}=(.*)$", text, re.M)
    if not m:
        return ""
    return m.group(1).strip().strip('"').strip("'")


def load_value(key: str) -> tuple[str, Path]:
    for p in TOKEN_FILES:
        v = _read_env_value(p, key)
        if v:
            return v, p
    raise SystemExit(f"{key} not found")


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
        try:
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
        except urllib.error.HTTPError as e:
            err = e.read().decode("utf-8", errors="replace")
            raise SystemExit(f"HTTP {e.code} {method}: {err[:800]}") from e

        payload = self._parse(raw)
        if "error" in payload:
            raise SystemExit(f"MCP error {method}: {payload['error']}")
        return payload

    @staticmethod
    def _parse(raw: str) -> dict:
        raw = raw.strip()
        if not raw:
            return {}
        if raw.startswith("{"):
            return json.loads(raw)
        data_lines = [line[5:].strip() for line in raw.splitlines() if line.startswith("data:")]
        if not data_lines:
            return {"raw": raw[:500]}
        return json.loads(data_lines[-1])

    def tool(self, name: str, arguments: dict) -> dict:
        return self.call("tools/call", {"name": name, "arguments": arguments})


def tool_text(result: dict) -> str:
    r = result.get("result") or {}
    content = r.get("content")
    if isinstance(content, list):
        return "\n".join(
            str(c.get("text", c)) if isinstance(c, dict) else str(c) for c in content
        )
    return json.dumps(r, ensure_ascii=False)


def redact_env_dump(text: str) -> str:
    # hide values after name= or "value":
    text = re.sub(r'("value"\s*:\s*")[^"]*', r'\1***', text)
    text = re.sub(r"(MAX_BOT_TOKEN\s*[:=]\s*)\S+", r"\1***", text, flags=re.I)
    return text[:1200]


def parse_env_entries(text: str) -> list[dict]:
    """Best-effort extract list of env var dicts from tool text."""
    # try JSON blob
    for m in re.finditer(r"\[.*\]|\{.*\}", text, re.S):
        try:
            obj = json.loads(m.group(0))
            if isinstance(obj, list):
                return [x for x in obj if isinstance(x, dict)]
            if isinstance(obj, dict):
                for k in ("variables", "envVars", "items", "data"):
                    if isinstance(obj.get(k), list):
                        return [x for x in obj[k] if isinstance(x, dict)]
        except json.JSONDecodeError:
            continue
    return []


def main() -> int:
    platform, psrc = load_value("AMVERA_PLATFORM_TOKEN")
    max_tok, msrc = load_value("MAX_BOT_TOKEN")
    print(f"platform_token_from={psrc}")
    print(f"max_token_from={msrc} len={len(max_tok)}")

    mcp = MCP(platform)
    mcp.call(
        "initialize",
        {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "uvo-set-max-token", "version": "1.0"},
        },
    )
    mcp.call("notifications/initialized", {}, notification=True)

    projects = tool_text(mcp.tool("listProjects", {}))
    print("projects:", redact_env_dump(projects).replace("\n", " | ")[:500])
    if SLUG not in projects.lower():
        # try getProject
        gp = tool_text(mcp.tool("getProject", {"slug": SLUG}))
        print("getProject:", redact_env_dump(gp)[:400])

    existing_id = None
    is_secret = True
    for secret_flag in (True, False):
        txt = tool_text(mcp.tool("listEnvVars", {"slug": SLUG, "isSecret": secret_flag}))
        # Do not dump values — only names/ids
        names = re.findall(r"ID=(\d+)\s+(\S+)=", txt)
        print(f"listEnvVars secret={secret_flag}: count={len(names)} names={[n for _, n in names[:20]]}")
        for vid, name in names:
            if name == ENV_KEY:
                existing_id = int(vid)
                is_secret = secret_flag
                print(f"found {ENV_KEY} id={existing_id} isSecret={is_secret}")
        for row in parse_env_entries(txt):
            name = str(row.get("name") or row.get("key") or "")
            if name == ENV_KEY and row.get("id") is not None:
                existing_id = int(row["id"])
                is_secret = bool(row.get("isSecret", secret_flag))
                print(f"found {ENV_KEY} id={existing_id} isSecret={is_secret}")

    if existing_id is not None:
        out = mcp.tool(
            "updateEnvVar",
            {
                "slug": SLUG,
                "id": int(existing_id),
                "name": ENV_KEY,
                "value": max_tok,
                "isSecret": True,
                "type": "RUN",
            },
        )
        print("updateEnvVar:", redact_env_dump(tool_text(out)))
    else:
        payload = json.dumps(
            [{"name": ENV_KEY, "value": max_tok, "isSecret": True, "type": "RUN"}],
            ensure_ascii=False,
        )
        out = mcp.tool("createEnvVars", {"slug": SLUG, "envVarsJson": payload})
        print("createEnvVars:", redact_env_dump(tool_text(out)))

    out = mcp.tool("restartProject", {"slug": SLUG})
    print("restartProject:", redact_env_dump(tool_text(out)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
