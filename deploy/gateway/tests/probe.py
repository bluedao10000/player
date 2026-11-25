"""Exercise the static site and real image origin through the TLS gateway."""
import base64
import hashlib
import http.client
import json
import re
import socket
import ssl
import sys
from pathlib import Path

http_port, https_port, image_port = map(int, sys.argv[1:4])
context = ssl.create_default_context(cafile=sys.argv[4])
site = "www.codeiy.com"
image = "img.codeiy.com"
sample = base64.b64decode(Path("apps/image/tests/sample.png.base64").read_bytes())
sha = hashlib.sha256(sample).hexdigest()
path = "/i/" + sha


class LocalTLS(http.client.HTTPSConnection):
    def connect(self):
        raw = socket.create_connection(("127.0.0.1", self.port), timeout=10)
        self.sock = self._context.wrap_socket(raw, server_hostname=self.host)


def fetch(conn, route, method="GET", body=None, headers=None):
    try:
        conn.request(method, route, body, headers or {})
        response = conn.getresponse()
        return response.status, {k.lower(): v for k, v in response.getheaders()}, response.read()
    finally:
        conn.close()


def https(host, route, *args, **kwargs):
    return fetch(LocalTLS(host, https_port, context=context), route, *args, **kwargs)


def admin(route, method, body=None, mime="application/json"):
    return fetch(http.client.HTTPConnection("localhost", image_port, timeout=30), route, method, body, {
        "Authorization": "Bearer test-token-of-at-least-thirty-two-characters",
        "Content-Type": mime,
    })


# A private digest is unresolvable even when its UUID has a signed URL.
status, _, body = admin("/v1/assets/upload?name=scene.png", "POST", sample, "image/png")
assert status == 201, (status, body)
asset = json.loads(body)["id"]
status, _, body = admin(f"/v1/assets/{asset}/sign?format=webp&w=6", "GET")
assert status == 200, (status, body)
signed = json.loads(body)["url"]
assert https(image, path)[0] == 404
for _ in range(2):
    status, headers, body = https(image, signed)
    assert (status, headers.get("x-cache"), headers["cache-control"], headers["content-type"]) == (
        200, "BYPASS", "private, no-store", "image/webp"
    ), headers

assert admin(f"/v1/assets/{asset}/publish", "POST", b'{"reference":"smoke-test"}')[0] == 200
status, headers, body = https(image, path, headers={"Origin": "https://" + site})
assert (status, body, headers["x-cache"], headers["content-type"]) == (200, sample, "MISS", "image/png"), headers
assert headers["access-control-allow-origin"] == "https://" + site and "access-control-allow-credentials" not in headers
assert headers["cross-origin-resource-policy"] == "cross-origin"
assert headers["cache-control"] == "public, max-age=31536000, immutable"
assert https(image, path)[1]["x-cache"] == "HIT"
assert https(image, path, "HEAD")[1]["etag"] == f'"{sha}"'
assert https(image, path, headers={"If-None-Match": f'"{sha}"'})[0] == 304
status, headers, body = https(image, path, headers={"Range": "bytes=1-5", "If-Range": f'"{sha}"'})
assert (status, headers["x-cache"], body) == (206, "BYPASS", sample[1:6]), headers
assert headers["content-range"] == f"bytes 1-5/{len(sample)}"
assert https(image, path, headers={"Authorization": "Bearer avoids shared cache"})[1]["x-cache"] == "BYPASS"
assert https(image, path, headers={"Cookie": "session=123"})[1]["x-cache"] == "BYPASS"
for fmt in ("webp", "avif"):
    variant = path + f"?w=6&h=4&fit=cover&crop=0,0,12,8&format={fmt}&q=70"
    status, headers, _ = https(image, variant)
    assert (status, headers.get("x-cache"), headers["content-type"]) == (200, "MISS", f"image/{fmt}"), headers
    assert headers["cache-control"] == "public, max-age=86400"
    assert https(image, variant)[1]["x-cache"] == "HIT"
assert https(image, path + "?sig=invalid&exp=1")[0] == 404
assert https(image, path, "POST")[0] == 405
assert https(image, "/v1/assets")[0] == 404
assert https(image, "/readyz")[0] == 200

status, headers, html = https(site, "/story/night-courier")
assert status == 200 and b'id="app"' in html
assert headers["cache-control"] == "no-cache, must-revalidate"
assert headers["strict-transport-security"] and headers["x-content-type-options"] == "nosniff"
assert https(site, "/index.html")[2] == html
assert https(site, "/work/tidepost/chapter/night-courier")[2] == html
javascript = re.search(rb'src="(/assets/[^"]+\.js)"', html)[1].decode()
status, headers, _ = https(site, javascript, headers={"Accept-Encoding": "gzip"})
assert status == 200 and headers["content-encoding"] == "gzip"
assert headers["cache-control"] == "public, max-age=31536000, immutable"
for route in ("/workbench", "/editor-api/account", "/api/v1/knowledge/scope-acceptances",
              "/assets/missing.js", "/art/night-post.webp", path, "/.env"):
    assert https(site, route)[0] == 404, route
for route in ("/healthz", "/readyz", "/api/v1/knowledge/discovery", "/api/v1/knowledge/works"):
    assert https(site, route)[0] == 200, route
assert https(site, "/api/v1/knowledge/discovery", "POST")[0] == 405
assert https(site, "/reader-auth/state")[2] == b"null"
status, headers, _ = https(site, "/reader-auth/wechat/start", "POST", b"{}", {
    "Content-Type": "application/json", "Origin": "https://" + site,
})
assert status == 200 and "HttpOnly" in headers["set-cookie"] and "Secure" in headers["set-cookie"], (status, headers)
status, _, body = https(site, "/ios-data/v1/discovery")
assert status == 200 and json.loads(body)["items"][0]["image"].startswith("https://" + image + "/i/")
assert https(site, "/api/billing/alipay/notify", "POST", b"out_trade_no=reader-order&test=%2B+%26", {
    "Content-Type": "application/x-www-form-urlencoded",
})[2] == b"success"
assert https(site, "/api/billing/alipay/notify")[0] == 405
assert https(site, "/api/billing/alipay/notify", "POST", b"x" * (1024 * 1024 + 1))[0] == 413
assert https(site, "/", headers={"Host": "unknown.example"})[0] == 404
assert https(image, path, headers={"Origin": "https://unknown.example"})[1]["access-control-allow-origin"] == "https://" + site
encoded_signature = signed.replace("&sig=", "&%73ig=")
assert https(image, encoded_signature)[1]["x-cache"] == "BYPASS"
status, headers, _ = fetch(http.client.HTTPConnection("localhost", http_port, timeout=10), "/story/night-courier",
                           headers={"Host": site})
assert status == 308 and headers["location"] == "https://" + site + "/story/night-courier"
try:
    https("unknown.example", "/")
except ssl.SSLError:
    pass
else:
    raise AssertionError("Unknown TLS hostname accepted")
print("PASS TLS, redirects, SPA fallback, static cache and gzip, Gin, iOS, OAuth and payment")
print("PASS Guangzhou image proxy, public cache, CORS, private bypass, Range, ETag, WebP and AVIF")
