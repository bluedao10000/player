"""Read published chapters and version-matched Bangumi schedules; emit one JSON snapshot."""
import datetime
import json
import os
import re
import subprocess
from pathlib import Path
from zoneinfo import ZoneInfo


def fetch(url):
    return json.loads(subprocess.check_output(
        ["curl", "--fail", "--silent", "--show-error", "--retry", "2", "--max-time", "25", url]
    ))


def schedule(subject):
    episodes = {}
    offset = 0
    while True:
        page = fetch(f"https://api.bgm.tv/v0/episodes?subject_id={subject}&type=0&limit=100&offset={offset}")
        for row in page["data"]:
            number = row["ep"]
            if number < 1 or int(number) != number or number in episodes:
                raise ValueError("季内集号需核对")
            episodes[int(number)] = datetime.date.fromisoformat(row["airdate"])
        offset += len(page["data"])
        if offset == page["total"]:
            return episodes
        if not page["data"] or offset > page["total"]:
            raise ValueError("分集目录在分页期间发生变化")


def episode_number(label):
    match = re.fullmatch(r"第\s*(\d+)\s*[集话]", label)
    if not match:
        raise ValueError(f"章节集号待核对 · {label}")
    return int(match[1])


def snapshot():
    now = datetime.datetime.now(ZoneInfo("Asia/Shanghai"))
    plans = Path(os.environ.get("EPISODE_CATALOG_DIR") or Path(__file__).resolve().parents[2] / "data/episode-plans")
    subjects = {"cowboy-bebop-tv": "253"}
    for path in plans.glob("*.json"):
        plan = json.loads(path.read_text())
        if plan["provider"] == "bangumi":
            subjects[plan["world"]] = plan["externalID"]
    rows = []
    for world in fetch("https://www.codeiy.com/ios-data/v1/discovery")["items"]:
        slug = world["slug"]
        row = {"slug": slug, "title": world["title"], "published": len(world["chapters"])}
        rows.append(row)
        if slug == "tidepost":
            row["note"] = "原创章节"
            continue
        try:
            subject = int(subjects[slug])
            row["source"] = f"https://bgm.tv/subject/{subject}/ep"
            episodes = schedule(subject)
            published = {1} if slug == "look-back-film-2024" and row["published"] == 1 else {
                episode_number(chapter["subtitle"]) for chapter in world["chapters"]
            }
            if not episodes or not published.issubset(episodes):
                raise ValueError("公开章节与季内分集目录不一致")
            row["missing"] = sorted(n for n,d in episodes.items() if d < now.date() and n not in published)
            row["today"] = sorted(n for n,d in episodes.items() if d == now.date() and n not in published)
        except (ValueError, KeyError, TypeError, subprocess.CalledProcessError) as err:
            row["note"] = f"分集目录待复核 · {err}"
    return {"checked": now.isoformat(timespec="seconds"), "works": rows}


if __name__ == "__main__":
    print(json.dumps(snapshot(), ensure_ascii=False))
