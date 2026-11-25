"""Render a single Nginx configuration from reviewed infrastructure settings."""
import argparse
import os
import re
from pathlib import Path


def setting(name, pattern, default=None):
    value = os.environ.get(name, default)
    if value is None or not re.fullmatch(pattern, value):
        raise SystemExit(f"Invalid or missing {name}")
    return value


def endpoint(name):
    value = setting(name, r"(?:unix:/[a-zA-Z0-9_./-]+\.sock|(?:[a-zA-Z0-9.-]+|\[[a-fA-F0-9:]+\]):[0-9]+)")
    if not value.startswith("unix:") and not 1 <= int(value.rsplit(":", 1)[1]) <= 65535:
        raise SystemExit(f"Invalid port in {name}")
    return value


parser = argparse.ArgumentParser()
parser.add_argument("directory", type=Path)
parser.add_argument("--docker", action="store_true")
options = parser.parse_args()
path_pattern = r"/[a-zA-Z0-9_./-]+"
hostname = r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?"
settings = {
    "PUBLIC_HOST": setting("PUBLIC_HOST", hostname, "www.codeiy.com"),
    "ACCOUNT_HOST": setting("ACCOUNT_HOST", hostname, "r.codeiy.com"),
    "API_UPSTREAM": endpoint("API_UPSTREAM"),
    "READER_API_UPSTREAM": endpoint("READER_API_UPSTREAM"),
    "IMAGE_UPSTREAM": endpoint("IMAGE_UPSTREAM"),
    "IMAGE_UPSTREAM_SCHEME": setting("IMAGE_UPSTREAM_SCHEME", r"https?", "http"),
    "IMAGE_UPSTREAM_HOST": setting("IMAGE_UPSTREAM_HOST", hostname),
    "HTTP_LISTEN": setting("HTTP_LISTEN", r"(?:(?:[0-9.]+|\[[a-fA-F0-9:]+\]):)?[0-9]{1,5}", "80"),
    "HTTPS_LISTEN": setting("HTTPS_LISTEN", r"(?:(?:[0-9.]+|\[[a-fA-F0-9:]+\]):)?[0-9]{1,5}", "443"),
}
for key, docker, native in [
    ("CONFIG_DIR", "/etc/folio-edge", str(options.directory.resolve())),
    ("TLS_FULLCHAIN", "/etc/folio-tls/fullchain.pem", None),
    ("TLS_PRIVATE_KEY", "/etc/folio-tls/privkey.pem", None),
    ("SITE_ROOT", "/usr/share/nginx/html", "/srv/folio-site"),
    ("IMAGE_CACHE_DIR", "/var/cache/nginx/folio-images", "/var/cache/nginx/folio-images"),
    ("NGINX_PID", "/run/nginx.pid", "/run/nginx.pid"),
    ("UPSTREAM_CA_BUNDLE", "/etc/ssl/certs/ca-certificates.crt", "/etc/ssl/certs/ca-certificates.crt"),
]:
    settings[key] = setting(key, path_pattern, docker if options.docker else native)
settings["NGINX_WORKER_USER"] = setting(
    "NGINX_WORKER_USER", r"[a-z_][a-z0-9_-]*", "folio-edge" if options.docker else None,
)
for name in ("HTTP_LISTEN", "HTTPS_LISTEN"):
    if not 1 <= int(settings[name].rsplit(":", 1)[-1]) <= 65535:
        raise SystemExit(f"Invalid port in {name}")
if settings["PUBLIC_HOST"] == "img.codeiy.com":
    raise SystemExit("PUBLIC_HOST must differ from the image host")

settings.update(TUNNEL_HTTP_CONFIG="", TUNNEL_HTTPS_REDIRECT="")
if setting("CLOUDFLARE_LOCAL_TUNNEL", r"[01]", "0") == "1":
    if any(not settings[key].startswith("127.0.0.1:") for key in ("HTTP_LISTEN", "HTTPS_LISTEN")):
        raise SystemExit("Cloudflare Tunnel requires both Nginx listeners on IPv4 loopback")
    settings["TUNNEL_HTTP_CONFIG"] = """set_real_ip_from 127.0.0.1;
    real_ip_header CF-Connecting-IP;
    map $http_x_forwarded_proto $redirect_tunnel_http { default 0; http 1; }"""
    settings["TUNNEL_HTTPS_REDIRECT"] = 'if ($redirect_tunnel_http) { return 308 https://$host$request_uri; }'

options.directory.mkdir(parents=True, exist_ok=True)
source = Path(__file__).resolve().parents[1] / "nginx"
for name in ("nginx.conf", "routes.conf"):
    template = (source / name).read_text()
    # Nginx uses $variables. Only substitute documented braced deployment variables.
    config = re.sub(r"\$\{([A-Z_]+)\}", lambda match: settings[match[1]], template)
    (options.directory / name).write_text(config)
