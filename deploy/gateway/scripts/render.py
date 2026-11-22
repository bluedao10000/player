"""Render a self-contained release from the operator's reviewed settings and asset manifest."""
import ipaddress
import os
import re
import sys
from pathlib import Path
from string import Template


def hostname(key):
    value = os.environ[key]
    if len(value) > 253 or not all(re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", label)
                                   for label in value.split(".")):
        raise ValueError(f"Invalid {key}")
    return value


def endpoint(key):
    value = os.environ[key]
    if value in {f"unix:/mesh/{name}.sock" for name in ("web", "api", "image")}:
        return value
    host, _, port = value.rpartition(":")
    if not ipaddress.IPv4Address(host).is_loopback or not port.isdecimal() or not 1 <= int(port) <= 65535:
        raise ValueError(f"{key} must use a Mesh socket or loopback")
    return value


src = Path(__file__).resolve().parents[1] / "nginx"
dest = Path(sys.argv[1])
if not re.fullmatch(r"[a-zA-Z0-9_-]+", dest.name):
    raise ValueError("Invalid release name")
assets = Path(os.environ["MANIFEST"]).read_text().splitlines()
if not assets or len(set(assets)) != len(assets):
    raise ValueError("Expected a nonempty manifest with unique asset paths")
routes = []
for asset in assets:
    if not re.fullmatch(r"art/(episode-scenes/[a-f0-9]{64}\.jpg|portraits/[a-f0-9]{64}\.webp)", asset):
        raise ValueError("Invalid pilot asset path")
    routes.append(f"""location = /{asset} {{
    limit_except GET {{ deny all; }}
    proxy_intercept_errors on;
    error_page 502 503 504 = @original;
    proxy_pass http://gray_origin;
}}
location = /image-preview/{asset} {{
    limit_except GET {{ deny all; }}
    proxy_hide_header Cache-Control;
    add_header Cache-Control "private, no-store" always;
    add_header Referrer-Policy no-referrer always;
    proxy_pass http://gray_origin;
}}""")

settings = dict(RELEASE=dest.name, PUBLIC_HOST=hostname("PUBLIC_HOST"), ACCOUNT_HOST=hostname("ACCOUNT_HOST"),
                CONFIG_DIR=f"/edge/{dest.name}", GRAY_ROUTES="\n".join(routes))
(dest / "public-host").write_text(settings["PUBLIC_HOST"] + "\n")
for name in ("nginx.conf", "routes.conf"):
    (dest / name).write_text(Template((src / name).read_text()).safe_substitute(settings))
with (dest / "upstreams.conf").open("w") as f:
    for name in ("web", "static", "api", "auth", "billing", "image", "gray", "fallback"):
        f.write(f"upstream {name}_origin {{ server {endpoint(name.upper() + '_UPSTREAM')}; keepalive 16; }}\n")
