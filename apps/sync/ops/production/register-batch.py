#!/usr/bin/python3
"""Register up to five explicitly staged episode plans after a fresh inventory."""
import json
import subprocess
import sys
from pathlib import Path


def main():
    directory = Path(sys.argv[1])
    registered = directory / "registered"
    registered.mkdir(exist_ok=True)
    remaining = 5
    failures = 0
    for path in sorted(directory.glob("*.json")):
        try:
            raw = path.read_bytes()
            plan = json.loads(raw)
            policy = plan["automation"]
            numbers = set(range(policy["first"], policy["last"] + 1)) if (
                type(policy["first"]) is int and type(policy["last"]) is int
                and 1 <= policy["first"] <= policy["last"] < policy["first"] + 5
            ) else set()
            episodes = plan["episodes"]
            if not numbers or len(episodes) != len(numbers) or {ep["number"] for ep in episodes} != numbers:
                raise ValueError("配置必须覆盖单个连续范围，且最多五集")
            if any(ep.get("publishToReader") is not False for ep in episodes):
                raise ValueError("批量登记要求 publishToReader 为 false")
            if (registered / path.name).exists():
                raise ValueError("文件名已登记，请使用唯一批次名")
            if len(numbers) > remaining:
                break
            remaining -= len(numbers)
            subprocess.run(["bin/knowledge", "episode-register"], input=raw, check=True)
            path.rename(registered / path.name)
            print(f"已登记 {path.name}，{len(numbers)} 集", flush=True)
        except (ValueError, KeyError, TypeError, OSError, subprocess.CalledProcessError) as err:
            print(f"登记失败 {path.name} · {err}", file=sys.stderr, flush=True)
            failures += 1
    return int(failures > 0)


if __name__ == "__main__":
    sys.exit(main())
