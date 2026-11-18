"""Check the repository handbook offline with Python's standard library."""
from html.parser import HTMLParser
from pathlib import Path
import re
from urllib.parse import unquote, urlsplit
from xml.etree import ElementTree

DOCS = Path(__file__).resolve().parent
ROOT = DOCS.parent


class HtmlLinks(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        self.links.extend(value for key, value in attrs if key in ("href", "src") and value)


def local_links(file):
    text = file.read_text()
    if file.suffix == ".html":
        parser = HtmlLinks()
        parser.feed(text)
        return parser.links
    prose = re.sub(r"^`{3}.*?^`{3}", "", text, flags=re.M | re.S)
    return re.findall(r"!?\[[^\]\n]*\]\(([^)\s]+)\)", prose)


def check():
    files = [ROOT / "AGENTS.md", ROOT / "README.md", *DOCS.rglob("*.md"), *DOCS.rglob("*.html")]
    errors = []
    for file in files:
        # Atlas template links resolve relative to the generated architecture.html.
        base = DOCS if file == DOCS / "diagrams/atlas.template.html" else file.parent
        for target in local_links(file):
            url = urlsplit(target)
            if url.scheme or url.netloc or not url.path or url.path.startswith("/"):
                continue
            if not (base / unquote(url.path)).exists():
                errors.append(f"{file.relative_to(ROOT)} -> {target}")

    sources = [DOCS / "architecture.mmd", *(DOCS / "diagrams").glob("*.mmd")]
    for source in sources:
        notes = re.findall(r"^%% (\w+) \| .+? \| (.+)$", source.read_text(), re.M)
        for _, file in notes:
            if not (ROOT / file).exists():
                errors.append(f"{source.name} -> {file}")
        name = "technology" if source.name == "architecture.mmd" else source.stem
        svg = ElementTree.parse(DOCS / "diagrams" / f"{name}.svg")
        rendered = {element.attrib["data-node"] for element in svg.iter() if "data-node" in element.attrib}
        if rendered != {key for key, _ in notes}:
            errors.append(f"{source.name} -> {name}.svg: rebuild the diagram")
    if errors:
        raise SystemExit("\n".join(errors))
    print(f"OK: {len(files)} handbook files, {len(sources)} architecture diagrams")


if __name__ == "__main__":
    check()
