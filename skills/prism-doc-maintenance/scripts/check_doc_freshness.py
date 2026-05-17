#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]

GENERATORS = [
    ROOT / "skills" / "prism-repo-map" / "scripts" / "extract_api_index.py",
    ROOT / "skills" / "prism-ai-runtime" / "scripts" / "extract_config_index.py",
]


def snapshot(paths):
    return {path: path.read_text(encoding="utf-8", errors="ignore") if path.exists() else None for path in paths}


def main() -> int:
    outputs = [
        ROOT / "skills" / "prism-repo-map" / "references" / "generated-api-index.md",
        ROOT / "skills" / "prism-ai-runtime" / "references" / "generated-config-index.md",
    ]
    before = snapshot(outputs)
    for generator in GENERATORS:
        result = subprocess.run([sys.executable, str(generator)], cwd=str(ROOT), text=True, capture_output=True)
        if result.returncode != 0:
            sys.stderr.write(result.stderr)
            return result.returncode
    after = snapshot(outputs)
    stale = [path for path in outputs if before.get(path) != after.get(path)]
    if stale:
        for path in stale:
            print(f"生成文档已刷新，请检查并提交：{path.relative_to(ROOT).as_posix()}")
        return 1
    print("生成文档是最新的")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
