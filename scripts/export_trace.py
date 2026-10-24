#!/usr/bin/env python3
"""Экспорт сессии Claude Code (JSONL) в читаемые трассы Markdown по этапам.

Использование:
    python3 scripts/export_trace.py ~/.claude/projects/<проект>/<session>.jsonl traces/

Что попадает в трассу: запросы пользователя, видимые ответы агента, вызовы
инструментов (команда / файл / описание) и их результаты (усечённые).
Внутренние рассуждения модели не экспортируются. Личные email и домашний
каталог заменяются заглушками.

Этапы определяются по времени первого коммита «stageN:» в git: событие
относится к этапу, начатому последним до него.
"""
import json
import re
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

ALLOWED_EMAIL_DOMAINS = {"vestnik.local", "v.ru", "x.ru", "example.org", "anthropic.com"}
EMAIL_RE = re.compile(r"[\w.+-]+@([\w-]+\.)+[\w-]+")
RESULT_LINES = 40
STAGES = {
    0: "Подготовка: чтение задания",
    1: "Этап 1. Предметная область и каркас",
    2: "Этап 2. Ядро управления контентом",
    3: "Этап 3. Панель администрирования",
    4: "Этап 4. Шаблонный слой и публичная часть",
    5: "Этап 5. Аналитика, релиз и оформление отчёта",
}


def scrub(text: str) -> str:
    def repl(m):
        domain = m.group(0).split("@", 1)[1].lower()
        return m.group(0) if domain in ALLOWED_EMAIL_DOMAINS else "<email>"

    text = EMAIL_RE.sub(repl, text)
    text = text.replace(str(Path.home()), "~")
    return text


def parse_ts(s: str) -> datetime:
    return datetime.fromisoformat(s.replace("Z", "+00:00")).astimezone(timezone.utc)


def stage_starts(repo: Path) -> list[tuple[datetime, int]]:
    """Этап 1 начинается за 5 минут до коммита init (спецификации писались до него),
    этап N+1 — в момент merge-коммита этапа N в main."""
    out = subprocess.run(["git", "-C", str(repo), "log", "--all", "--reverse", "--format=%cI %s"],
                         capture_output=True, text=True, check=True).stdout
    starts = {}
    for line in out.splitlines():
        ts, _, subject = line.partition(" ")
        if subject.startswith("init:") and 1 not in starts:
            starts[1] = parse_ts(ts) - timedelta(minutes=5)
        m = re.match(r"Merge feature/0(\d)", subject)
        if m and int(m.group(1)) + 1 <= 5:
            starts.setdefault(int(m.group(1)) + 1, parse_ts(ts))
    return sorted((t, n) for n, t in starts.items())


def stage_of(ts: datetime, starts) -> int:
    current = 0
    for start, n in starts:
        if ts >= start:
            current = n
    return current


def tool_input_summary(name: str, inp: dict) -> str:
    parts = []
    if inp.get("description"):
        parts.append(f"_{inp['description']}_")
    if "command" in inp:
        parts.append("```bash\n" + inp["command"].strip()[:1500] + "\n```")
    elif "file_path" in inp:
        parts.append(f"`{inp['file_path']}`")
        if name == "Edit":
            old = inp.get("old_string", "").strip().splitlines()
            new = inp.get("new_string", "").strip().splitlines()
            parts.append(f"замена {len(old)} строк на {len(new)}")
        elif name == "Write":
            parts.append(f"{len(inp.get('content', '').splitlines())} строк")
    elif "skill" in inp:
        parts.append(f"skill `{inp['skill']}`")
    else:
        parts.append("```json\n" + json.dumps(inp, ensure_ascii=False)[:500] + "\n```")
    return "\n".join(parts)


def result_text(content) -> str:
    if isinstance(content, list):
        chunks = []
        for c in content:
            if c.get("type") == "text":
                chunks.append(c["text"])
            elif c.get("type") == "image":
                chunks.append("[изображение — скриншот]")
        content = "\n".join(chunks)
    lines = str(content).splitlines()
    if len(lines) > RESULT_LINES:
        lines = lines[:RESULT_LINES] + [f"… (ещё {len(lines) - RESULT_LINES} строк)"]
    return "\n".join(lines)


def main():
    src, out_dir = Path(sys.argv[1]), Path(sys.argv[2])
    repo = Path(__file__).resolve().parent.parent
    starts = stage_starts(repo)
    out_dir.mkdir(parents=True, exist_ok=True)
    events = {n: [] for n in STAGES}
    counts = {n: {"tool_calls": 0, "errors": 0} for n in STAGES}
    session = None
    for raw in src.open(encoding="utf-8"):
        d = json.loads(raw)
        if d.get("type") not in ("user", "assistant") or d.get("isSidechain"):
            continue
        session = session or d.get("sessionId")
        ts = parse_ts(d["timestamp"])
        st = stage_of(ts, starts)
        msg = d.get("message", {})
        content = msg.get("content")
        stamp = ts.astimezone().strftime("%H:%M:%S")
        if d["type"] == "user" and isinstance(content, str):
            if content.startswith("<local-command") or "<command-name>" in content:
                continue
            events[st].append(f"### {stamp} — запрос пользователя\n\n> " + scrub(content.strip()).replace("\n", "\n> "))
            continue
        if not isinstance(content, list):
            continue
        for c in content:
            kind = c.get("type")
            if kind == "text" and d["type"] == "assistant" and c["text"].strip():
                events[st].append(f"### {stamp} — агент\n\n" + scrub(c["text"].strip()))
            elif kind == "tool_use":
                counts[st]["tool_calls"] += 1
                events[st].append(f"#### {stamp} — инструмент `{c['name']}`\n\n" + scrub(tool_input_summary(c["name"], c.get("input", {}))))
            elif kind == "tool_result":
                err = c.get("is_error")
                if err:
                    counts[st]["errors"] += 1
                text = scrub(result_text(c.get("content", "")))
                label = "ошибка" if err else "результат"
                events[st].append(f"<details><summary>{label}</summary>\n\n```\n{text}\n```\n</details>")
            elif kind == "text" and d["type"] == "user" and c["text"].strip() and not c["text"].startswith("<"):
                events[st].append(f"### {stamp} — запрос пользователя\n\n> " + scrub(c["text"].strip()).replace("\n", "\n> "))

    index = [f"# Трассы агентской сессии `{session}`\n",
             "Экспорт из журнала Claude Code скриптом `scripts/export_trace.py`. Рассуждения модели не включены.\n",
             "| Этап | Файл | Вызовов инструментов | Ошибок инструментов |", "|---|---|---|---|"]
    for n, title in STAGES.items():
        name = f"stage-{n}.md"
        body = f"# {title}\n\nСессия `{session}`. Время — местное.\n\n" + "\n\n".join(events[n]) + "\n"
        (out_dir / name).write_text(body, encoding="utf-8")
        index.append(f"| {title} | [{name}]({name}) | {counts[n]['tool_calls']} | {counts[n]['errors']} |")
    (out_dir / "INDEX.md").write_text("\n".join(index) + "\n", encoding="utf-8")
    print("\n".join(index))


if __name__ == "__main__":
    main()
