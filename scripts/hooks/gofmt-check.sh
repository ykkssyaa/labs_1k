#!/usr/bin/env bash
# PostToolUse: если агент записал .go-файл без gofmt — сообщить ему сразу.
set -euo pipefail
path=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))')
[[ "$path" == *.go && -f "$path" ]] || exit 0
if [[ -n "$(gofmt -l "$path")" ]]; then
  echo "Файл $path не отформатирован gofmt. Запусти gofmt -w $path." >&2
  exit 2
fi
