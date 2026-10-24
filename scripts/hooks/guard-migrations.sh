#!/usr/bin/env bash
# PreToolUse: запрещает агенту править уже закоммиченные миграции БД.
# Код выхода 2 блокирует вызов инструмента, stderr уходит агенту как объяснение.
set -euo pipefail
input=$(cat)
path=$(printf '%s' "$input" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))')
case "$path" in
  */internal/store/migrations/*.sql)
    root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
    rel="${path#"$root"/}"
    if git -C "$root" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
      echo "Миграция $rel уже в git и могла быть применена. Не меняй её — создай новую миграцию с следующим номером." >&2
      exit 2
    fi
    ;;
esac
exit 0
