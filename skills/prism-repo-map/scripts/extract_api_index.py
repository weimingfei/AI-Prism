#!/usr/bin/env python3
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "skills" / "prism-repo-map" / "references" / "generated-api-index.md"
API_ROOT = ROOT / "backend" / "apps"
ROUTE_RE = re.compile(r'router\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)",\s*h\.([A-Za-z0-9_]+)\)')


def main() -> None:
    rows = []
    for path in sorted(API_ROOT.glob("*/api/*.go")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        for match in ROUTE_RE.finditer(text):
            method, route, handler = match.groups()
            rows.append((path.relative_to(ROOT).as_posix(), method, route, handler))

    lines = [
        "<!-- GENERATED: run `python skills/prism-repo-map/scripts/extract_api_index.py` from repo root. Do not edit by hand. -->",
        "",
        "# 后端 API 索引",
        "",
        "| 文件 | 方法 | 路由 | Handler |",
        "| --- | --- | --- | --- |",
    ]
    for file, method, route, handler in rows:
        lines.append(f"| `{file}` | `{method}` | `{route}` | `{handler}` |")
    OUT.write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
