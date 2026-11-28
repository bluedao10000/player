"""Private upstream fixture for the gateway smoke test."""
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

world = {
    "slug": "tidepost",
    "title": "白帆港",
    "edition_label": "原创",
    "intro": "广州读者数据",
    "origin": "original",
    "href": "/work/tidepost",
    "hero_asset_key": "/art/night-post.webp",
}
chapter = {
    "slug": "night-courier", "title": "夜班信使", "label": "第一章",
    "intro": "白帆港的夜晚", "image": "/art/night-post.webp", "href": "/story/night-courier",
}


class Service(BaseHTTPRequestHandler):
    def reply(self, code, data, headers=None):
        self.send_response(code)
        for k, v in (headers or {}).items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(data)

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path == "/api/v1/knowledge/discovery":
            data = {"items": [{**world, "characters": [], "chapters": [chapter], "scene": None}]}
        elif path == "/api/v1/knowledge/worlds/tidepost/chapters/night-courier":
            data = {
                "work": world, "chapter": chapter,
                "blocks": [{"key": "first", "text": "测试章节正文", "anchors": ["passage-unstamped-letter"]}],
                "scenes": [], "chapter_cast": [],
            }
        elif path == "/api/v1/knowledge/worlds/tidepost/chapters/legacy":
            data = {
                "work": world, "chapter": {**chapter, "slug": "legacy", "href": "/work/tidepost/chapter/legacy"},
                "research": {"paragraphs": [{"text": "旧版导读正文"}]},
                "blocks": [{"key": "pier", "text": "码头的场景", "anchors": ["passage-pier", "old-pier"]}],
                "scenes": [], "attribution": [],
            }
        elif path == "/api/v1/knowledge/worlds/tidepost/scenes/unstamped-letter":
            data = {
                "work": world, "chapter": chapter, "scene": {"title": "邮局的灯", "place": "邮局", "text": "回到章节"},
                "scene_cast": [], "attribution": [],
                "return_to": {"href": "/story/night-courier#passage-unstamped-letter"},
            }
        elif path == "/api/v1/knowledge/worlds/tidepost":
            data = {"work": world, "storylines": [], "characters": [], "scenes": []}
        elif path == "/api/v1/knowledge/works":
            data = {"items": [], "next_cursor": None}
        else:
            self.reply(404, b"")
            return
        self.reply(200, json.dumps(data).encode(), {"Content-Type": "application/json"})

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path == "/api/billing/alipay/notify":
            self.reply(200 if body == b"out_trade_no=reader-order&test=%2B+%26" else 422, b"success")
        elif self.path == "/api/v1/reader/wechat/start":
            self.reply(200, json.dumps({"proof": "a" * 64, "url": "https://open.weixin.qq.com/test"}).encode())
        else:
            self.reply(404, b"")


ThreadingHTTPServer(("0.0.0.0", 8080), Service).serve_forever()
