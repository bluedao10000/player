"""Exercise the rendered Nginx configuration with private, deterministic fixture origins."""
import hashlib
import http.client
import http.server
import json
import socket
import sys
import threading
import time
from urllib.parse import urlsplit

asset = "/art/episode-scenes/" + "0" * 64 + ".jpg"
event_read = threading.Event()


class Origin(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        path = urlsplit(self.path)
        if path.path == "/stream":
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(b"data: first\n\n")
            self.wfile.flush()
            if event_read.wait(5):
                self.wfile.write(b"data: last\n\n")
            self.close_connection = True
            return
        if path.path == "/ack":
            event_read.set()
        if self.headers.get("Upgrade") == "websocket":
            import base64
            self.send_response(101)
            self.send_header("Upgrade", "websocket")
            self.send_header("Connection", "Upgrade")
            key = self.headers["Sec-WebSocket-Key"] + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
            self.send_header("Sec-WebSocket-Accept", base64.b64encode(hashlib.sha1(key.encode()).digest()).decode())
            self.end_headers()
            self.wfile.write(b"\x81\x02ok")
            self.close_connection = True
            return
        port = self.server.server_port
        status = 200
        body = b"ok"
        headers = {}
        if path.path == asset and port == 18007 and path.query:
            status = int(path.query)
        if path.path.startswith("/image-preview/"):
            status = 403
        if path.path.startswith("/i/"):
            headers.update(ETag='"fixture"', **{"Cache-Control": "public, max-age=31536000, immutable"})
            if self.headers.get("Range") == "bytes=0-0":
                status, body = 206, b"o"
                headers["Content-Range"] = "bytes 0-0/2"
            if self.headers.get("If-None-Match") == '"fixture"':
                status, body = 304, b""
        if path.path.startswith(("/reader-auth/", "/ios-account/", "/api/billing/")):
            if self.headers.get("Transfer-Encoding") == "chunked":
                parts = []
                while size := int(self.rfile.readline(), 16):
                    parts.append(self.rfile.read(size))
                    assert self.rfile.read(2) == b"\r\n"
                self.rfile.readline()
                raw = b"".join(parts)
            else:
                raw = self.rfile.read(int(self.headers.get("Content-Length", 0)))
            body = json.dumps([dict(self.headers), raw.decode()]).encode()
        self.send_response(status)
        self.send_header("X-Test-Origin", str(port))
        self.send_header("Content-Length", str(len(body)))
        for key, value in headers.items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(body)

    do_POST = do_GET

    def log_message(self, *args):
        pass


class Client(http.client.HTTPConnection):
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.settimeout(8)
        self.sock.connect("/run/player-edge/" + self.socket_name)

    socket_name = "origin.sock"


def fetch(path, method="GET", body=None, **headers):
    conn = Client("www.codeiy.com")
    conn.request(method, path, body, headers)
    response = conn.getresponse()
    result = response.status, response.headers, response.read()
    conn.close()
    return result


def check():
    for _ in range(50):
        try:
            if fetch("/")[0] == 200:
                break
        except OSError:
            pass
        time.sleep(0.1)
    for path, port in [("/work/example", 18001), ("/story/example", 18001), ("/ios-data/v1/discovery", 18001),
                       ("/assets/a.js", 18002), ("/api/v1/knowledge/discovery", 18003), (asset, 18007)]:
        status, headers, _ = fetch(path)
        assert (status, headers["X-Test-Origin"]) == (200, str(port)), path
    for path in ["/workbench", "/workbench-art/test", "/editor-api/x", "/api/v1/admin/knowledge",
                 "/api/v1/knowledge/research-runs", "/api/v1/reader/workbench/code", "/v1/assets", "/image-preview/other"]:
        assert fetch(path)[0] == 404, path
    assert fetch("/", Host="foreign.invalid")[0] == 404
    for provider in ["wechat", "github"]:
        path = "/api/auth/callback/" + provider + "?state=x"
        status, headers, _ = fetch(path)
        assert (status, headers["Location"]) == (302, "https://localhost" + path)
        assert fetch(path, "POST")[0] == 405
    token = "a" * 64
    spoof = {"Cookie": "other=bad; folio_reader=" + token + "; folio_flow=" + token,
             "Authorization": "Bearer native", "CF-Connecting-IP": "192.0.2.42", "Forwarded": "bad",
             "X-Forwarded-For": "bad", "X-Forwarded-Proto": "http", "X-Forwarded-Extra": "bad"}
    for path, port in [("/ios-account/state", 18004), ("/reader-auth/callback/wechat", 18004),
                       ("/api/billing/alipay/notify", 18005)]:
        data = "out_trade_no=a%2Bb&x=1&x=2"
        post = "notify" in path
        status, reply, body = fetch(path, "POST" if post else "GET", data if post else None,
                                    **spoof, **{"Content-Type": "application/x-www-form-urlencoded"})
        assert (status, reply["X-Test-Origin"]) == (200, str(port))
        hdr, received = json.loads(body)
        hdr = {k.lower(): v for k, v in hdr.items()}
        assert received == (data if post else "")
        assert hdr["x-forwarded-for"] == "192.0.2.42" and hdr["x-forwarded-proto"] == "https"
        assert not {"forwarded", "x-forwarded-extra", "cf-connecting-ip"} & hdr.keys()
        assert hdr.get("authorization") == ("Bearer native" if path.startswith("/ios-account") else None)
        assert hdr.get("cookie") == (f"folio_reader={token}; folio_flow={token}" if "wechat" in path else None)
    status, _, raw = fetch("/api/billing/alipay/notify", "POST", iter([b"x=a%2B", b"b&x=2"]))
    assert status == 200 and json.loads(raw)[1] == "x=a%2Bb&x=2"
    for code in [502, 503, 504]:
        assert fetch(f"{asset}?{code}")[1]["X-Test-Origin"] == "18008"
    assert fetch(asset + "?404")[0] == 404
    status, headers, _ = fetch("/image-preview" + asset)
    assert status == 403 and headers["Cache-Control"] == "private, no-store"
    status, headers, body = fetch("/i/example", Range="bytes=0-0")
    assert status == 206 and body == b"o" and headers["ETag"] == '"fixture"'
    assert headers["Content-Range"] == "bytes 0-0/2" and "immutable" in headers["Cache-Control"]
    assert fetch("/i/example", **{"If-None-Match": '"fixture"'})[0] == 304
    stream = Client("www.codeiy.com")
    stream.request("GET", "/stream")
    response = stream.getresponse()
    assert response.readline() == b"data: first\n"
    fetch("/ack")
    assert b"data: last" in response.read()
    stream.close()
    ws = Client("www.codeiy.com")
    ws.connect()
    ws.sock.sendall(b"GET /ws HTTP/1.1\r\nHost: www.codeiy.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
                    b"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
    buf = ws.sock.makefile("rb")
    assert b" 101 " in buf.readline()
    handshake = http.client.parse_headers(buf)
    assert handshake["Sec-WebSocket-Accept"] == "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
    assert buf.read(4) == b"\x81\x02ok"
    ws.close()
    print("PASS routing, headers, callbacks, POST bytes, private paths, gray fallback, Range/ETag, SSE and WebSocket")


if sys.argv[1] == "serve":
    for port in range(18001, 18009):
        app = http.server.ThreadingHTTPServer(("127.0.0.1", port), Origin)
        threading.Thread(target=app.serve_forever, daemon=True).start()
    threading.Event().wait()
elif sys.argv[1] == "version":
    Client.socket_name = "check.sock"
    for _ in range(50):
        if fetch("/")[2].decode() == sys.argv[2]:
            break
        time.sleep(.1)
    else:
        raise AssertionError("Reload did not switch release")
    print("PASS loaded release", sys.argv[2])
else:
    check()
