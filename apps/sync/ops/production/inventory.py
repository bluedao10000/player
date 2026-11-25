"""Read published chapters and version-matched Bangumi schedules; emit one JSON snapshot."""
import datetime
import json
import os
import re
import subprocess
from pathlib import Path
from zoneinfo import ZoneInfo


def fetch(url):
    command = ["curl", "--fail", "--silent", "--show-error", "--retry", "2",
               "--connect-timeout", "8", "--max-time", "25"]
    return json.loads(subprocess.check_output([*command, url]))


def schedule_page(subject, offset):
    ssh_config = os.environ.get("BANGUMI_SSH_CONFIG")
    if ssh_config:
        return json.loads(subprocess.check_output(
            ["ssh", "-F", ssh_config, "bangumi-inventory", str(subject), str(offset)],
            timeout=40,
        ))
    return fetch(f"https://api.bgm.tv/v0/episodes?subject_id={subject}&type=0&limit=100&offset={offset}")


def schedule(subject):
    episodes = {}
    offset = 0
    while True:
        page = schedule_page(subject, offset)
        for row in page["data"]:
            number = row["ep"]
            if number < 1 or int(number) != number or number in episodes:
                raise ValueError("季内集号需核对")
            airdate = row.get("airdate")
            episodes[int(number)] = (
                datetime.date.fromisoformat(airdate) if airdate and airdate != "0000-00-00" else None
            )
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
    plans = Path(os.environ.get("EPISODE_CATALOG_DIR") or Path.home() / ".local/share/folio/episode-plans")
    subjects = {"cowboy-bebop-tv": "253"}
    for path in plans.glob("*.json"):
        plan = json.loads(path.read_text())
        if plan["provider"] == "bangumi":
            subjects[plan["world"]] = plan["externalID"]
    rows = []
    for world in fetch(os.environ["READER_PUBLIC_URL"].rstrip("/") + "/ios-data/v1/discovery")["items"]:
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
            row["catalogEpisodes"] = len(episodes)
            row["missing"] = sorted(n for n,d in episodes.items() if d is not None and d < now.date() and n not in published)
            row["today"] = sorted(n for n,d in episodes.items() if d == now.date() and n not in published)
            row["future"] = sorted(n for n,d in episodes.items() if d is not None and d > now.date() and n not in published)
            row["undated"] = sorted(n for n,d in episodes.items() if d is None and n not in published)
        except (ValueError, KeyError, TypeError, subprocess.SubprocessError) as err:
            row["note"] = f"分集目录待复核 · {err}"
    if any(row["slug"] in subjects for row in rows) and not any("missing" in row for row in rows):
        raise RuntimeError("Bangumi 分集盘点全部失败，保留上次快照")
    return {"checked": now.isoformat(timespec="seconds"), "works": rows}


if __name__ == "__main__":
    print(json.dumps(snapshot(), ensure_ascii=False))
