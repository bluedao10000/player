"""Issue a five-minute preview for an imported asset on the Guangzhou origin."""
import hashlib
import hmac
import pathlib
import sys
import time

root = pathlib.Path("/var/lib/folio-image")
asset = sys.argv[1]
if asset not in (root / "manifest.txt").read_text().splitlines():
    raise SystemExit("Unknown asset")
path = "/image-preview/" + asset
expires = int(time.time()) + 300
signature = hmac.digest((root / "preview.key").read_bytes(),
                        f"{path}\n{expires}".encode(), hashlib.sha256).hex()
print(f"https://www.codeiy.com{path}?exp={expires}&sig={signature}")
