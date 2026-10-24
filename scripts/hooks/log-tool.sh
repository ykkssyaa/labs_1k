#!/usr/bin/env bash
# PostToolUse: краткий журнал вызовов инструментов агента в traces/tool-log.jsonl.
set -euo pipefail
root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
mkdir -p "$root/traces"
python3 -c '
import json, sys, datetime
e = json.load(sys.stdin)
ti = e.get("tool_input", {})
brief = ti.get("command") or ti.get("file_path") or ti.get("pattern") or ""
print(json.dumps({
  "ts": datetime.datetime.now().isoformat(timespec="seconds"),
  "session": e.get("session_id"),
  "tool": e.get("tool_name"),
  "input": str(brief)[:300],
}, ensure_ascii=False))
' >> "$root/traces/tool-log.jsonl"
