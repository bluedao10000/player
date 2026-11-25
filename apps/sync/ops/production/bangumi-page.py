#!/usr/bin/python3
"""Serve a public Bangumi episode page through a restricted SSH key."""
import os
import re
import sys


def main():
    command = os.environ.get("SSH_ORIGINAL_COMMAND", "")
    page = re.fullmatch(r"([1-9][0-9]{0,8}) ([0-9]{1,6})", command)
    if not page:
        sys.exit("Expected a Bangumi subject ID and episode offset")
    subject, offset = page.groups()
    url = f"https://api.bgm.tv/v0/episodes?subject_id={subject}&type=0&limit=100&offset={offset}"
    os.execv("/usr/bin/curl", [
        "curl", "--disable", "--fail", "--silent", "--show-error", "--retry", "2",
        "--retry-max-time", "25", "--connect-timeout", "5", "--max-time", "10",
        "--user-agent", "FolioInventory/1.0 (https://www.codeiy.com)", url,
    ])


if __name__ == "__main__":
    main()
