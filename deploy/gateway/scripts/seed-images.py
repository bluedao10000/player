"""Publish repository artwork and selected reader originals to the Guangzhou catalog."""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import urlencode, urlsplit
from urllib.request import build_opener, HTTPRedirectHandler, Request


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


parser = argparse.ArgumentParser()
parser.add_argument("references", type=Path, help="one published reference per line from published-images.sql")
parser.add_argument("--scene-archive", required=True, type=Path, help="directory containing selected JPEGs by SHA-256")
parser.add_argument("--portrait-archive", required=True, type=Path, help="directory containing selected WebP originals")
parser.add_argument("--verify-only", action="store_true")
options = parser.parse_args()

root = Path(__file__).resolve().parents[3] / "apps/web/public"
bundled = {
    "/" + p.relative_to(root).as_posix(): p
    for p in root.rglob("*") if p.suffix in (".png", ".webp") and p.is_file()
}
references = set(options.references.read_text().splitlines())
assets = {path: None for path in bundled.values()}
for reference in sorted(references - bundled.keys()):
    match = re.fullmatch(r"/art/(episode-scenes|portraits)/([a-f0-9]{64})\.(jpg|webp)", reference)
    if not match:
        raise SystemExit(f"Unknown published image reference: {reference}")
    kind, digest, ext = match.groups()
    if (kind, ext) == ("episode-scenes", "jpg"):
        path = options.scene_archive / digest
    elif (kind, ext) == ("portraits", "webp"):
        path = options.portrait_archive / (digest + ".webp")
    else:
        raise SystemExit(f"Unexpected archive format: {reference}")
    assets[path] = digest

# Validate the entire inventory before performing writes.
inventory = {}
for path, expected in assets.items():
    data = path.read_bytes()
    sha = hashlib.sha256(data).hexdigest()
    if expected and expected != sha:
        raise SystemExit(f"SHA-256 mismatch: {path}")
    if data.startswith(b"\xff\xd8"):
        mime, extension = "image/jpeg", "jpg"
    elif data.startswith(b"\x89PNG\r\n\x1a\n"):
        mime, extension = "image/png", "png"
    elif data.startswith(b"RIFF") and data[8:12] == b"WEBP":
        mime, extension = "image/webp", "webp"
    else:
        raise SystemExit(f"Unsupported original: {path}")
    inventory[sha] = (path, mime, extension)

base = os.environ["IMAGE_ADMIN_URL"].rstrip("/")
url = urlsplit(base)
if url.scheme not in ("http", "https") or not url.hostname or url.username or url.path or url.query or url.fragment:
    raise SystemExit("IMAGE_ADMIN_URL must be the private image service origin")
token = os.environ["IMAGE_API_TOKEN"] if not options.verify_only else ""
if not options.verify_only and not token:
    raise SystemExit("IMAGE_API_TOKEN is required")
http = build_opener(NoRedirect())


def call(method, path, body=None, mime=None):
    headers = {"Content-Type": mime, "Authorization": "Bearer " + token} if mime else {}
    return http.open(Request(base + path, method=method, data=body, headers=headers), timeout=120)


def available(sha):
    try:
        with call("HEAD", "/i/" + sha) as response:
            if response.status != 200 or response.headers["ETag"] != f'"{sha}"':
                raise SystemExit(f"Unexpected image response for {sha}")
            return True
    except HTTPError as error:
        if error.code == 404:
            return False
        raise


for sha, (path, mime, extension) in sorted(inventory.items()):
    if available(sha):
        continue
    if options.verify_only:
        raise SystemExit(f"Published image missing: {sha}")
    data = path.read_bytes()
    if hashlib.sha256(data).hexdigest() != sha:
        raise SystemExit(f"Original changed during import: {path}")
    query = urlencode({"name": sha + "." + extension, "source": "reader-inventory"})
    with call("POST", "/v1/assets/upload?" + query, data, mime) as response:
        result = json.load(response)
        if response.status != 201 or result.get("sha256") != sha:
            raise SystemExit(f"Upload checksum mismatch: {path}")
        asset = result["id"]
        if not re.fullmatch(r"[a-f0-9-]{36}", asset):
            raise SystemExit("Invalid image asset ID")
    with call("POST", f"/v1/assets/{asset}/publish", json.dumps({"reference": "reader-inventory:" + sha}).encode(),
              "application/json") as response:
        if response.status != 200:
            raise SystemExit(f"Publication failed: {path}")
    if not available(sha):
        raise SystemExit(f"Published image missing: {sha}")

print(f"Verified {len(inventory)} published images")
