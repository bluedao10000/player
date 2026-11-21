"""Fetch the Naruto pilot's published assets, pin original bytes and produce a gateway allowlist."""
import concurrent.futures
import hashlib
import json
import pathlib
import re
import os
import sys
import subprocess

site = os.environ["READER_PUBLIC_URL"].rstrip("/")
root = pathlib.Path(sys.argv[1])
world = os.environ["PILOT_WORK_SLUG"]

def fetch(path):
    return subprocess.check_output(["curl", "-fsS", "--retry", "2", "--max-time", "40", site + path])

def images(obj):
    if isinstance(obj, dict):
        for value in obj.values():
            yield from images(value)
    elif isinstance(obj, list):
        for value in obj:
            yield from images(value)
    elif isinstance(obj, str) and re.fullmatch(r"/art/(episode-scenes/[0-9a-f]{64}\.jpg|portraits/[0-9a-f]{64}\.webp)", obj):
        yield obj

def work(slug):
    return json.loads(fetch("/ios-data/v1/works/" + slug))

data = work(world)
paths = set(images(data))
for story in data["stories"]:
    for ch in story["chapters"]:
        paths.update(images(json.loads(fetch("/ios-data/v1/works/" + world + "/chapters/" + ch["slug"]))))
others = json.loads(fetch("/ios-data/v1/discovery"))["items"]
with concurrent.futures.ThreadPoolExecutor(6) as pool:
    for other in pool.map(work, (w["slug"] for w in others if w["slug"] != world)):
        if paths.intersection(images(other)):
            raise SystemExit("Shared asset requires separate rollout")
if not paths or not any("/portraits/" in p for p in paths):
    raise SystemExit("Incomplete public library")
for path in sorted(paths):
    dest = root / "library" / path.lstrip("/")
    dest.parent.mkdir(parents=True, exist_ok=True)
    body = fetch(path)
    if hashlib.sha256(body).hexdigest() != dest.stem:
        raise SystemExit("Unexpected digest: " + path)
    dest.write_bytes(body)
(root / "manifest.txt").write_text("".join(p.lstrip("/") + "\n" for p in sorted(paths)))

template = pathlib.Path(__file__).resolve().parents[4] / "deploy/cloudflare/player-ingress.conf"
pattern = "(?:" + "|".join(re.escape(path.lstrip("/")) for path in sorted(paths)) + ")"
(root / "player-ingress.conf").write_text(template.read_text().replace("__PUBLIC_ASSET_PATHS__", pattern))
