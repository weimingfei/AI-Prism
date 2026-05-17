#!/usr/bin/env python3
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[3]
CONFIG_GO = ROOT / "backend" / "internal" / "config" / "config.go"
APP_TOML = ROOT / "backend" / "etc" / "application.toml"
OUT = ROOT / "skills" / "prism-ai-runtime" / "references" / "generated-config-index.md"

TYPE_RE = re.compile(r"type\s+(\w+Config|Config)\s+struct\s*\{(?P<body>.*?)\n\}", re.S)
FIELD_RE = re.compile(r"^\s*(\w+)\s+[\w\[\]\*\.]+(?:\s+`toml:\"([^\"]+)\"`)?", re.M)
SECTION_RE = re.compile(r"^\[([^\]]+)\]", re.M)


def main() -> None:
    config_text = CONFIG_GO.read_text(encoding="utf-8", errors="ignore")
    toml_text = APP_TOML.read_text(encoding="utf-8", errors="ignore") if APP_TOML.exists() else ""

    lines = [
        "<!-- GENERATED: run `python skills/prism-ai-runtime/scripts/extract_config_index.py` from repo root. Do not edit by hand. -->",
        "",
        "# 配置索引",
        "",
        "## TOML 配置段",
        "",
    ]
    sections = SECTION_RE.findall(toml_text)
    for section in sections:
        lines.append(f"- `[{section}]`")

    lines += ["", "## Go 配置结构体字段", "", "| 结构体 | 字段 | TOML key |", "| --- | --- | --- |"]
    for type_match in TYPE_RE.finditer(config_text):
        type_name = type_match.group(1)
        for field, toml_key in FIELD_RE.findall(type_match.group("body")):
            lines.append(f"| `{type_name}` | `{field}` | `{toml_key or '-'}` |")

    OUT.write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
