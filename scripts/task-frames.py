#!/usr/bin/env python3
"""Redraw how the task manager marks each app in a bottom panel.

Usage: scripts/task-frames.py [UNDERLINE_SVG BACKGROUND_SVG]

The task manager draws a frame from widgets/tasks.svg behind each app in the
panel, picked by the app's state: normal (running), focus (its window is
active), minimized, attention (it's asking for attention), and nothing for an
app that's pinned but not running. When the pointer is over an app, Plasma
first looks for "<state>-hover", then the shared "hover". For a bottom panel
it uses these plain names; panels on other edges look for north-, east- or
west- versions first.

This script redraws the plain frames for each style in STYLES below, each a
tint behind the app and a 2 px line along the bottom edge:

  underline   the Plasma Style's own widgets/tasks.svg
  background  theme/variants/tasks-background.svg, which the installer puts
              in its place when chosen

The background one starts as a copy of the underline one. Each frame keeps
its size and the 1 px it leaves clear on each side, so the icons stay where
they are and neighbouring apps sit 2 px apart. Each file is marked with its
style (<desc id="protea-task-style">), which is how the installer tells
which one is installed.

It also gives top and side panels a copy of their own hover frame for each
"<state>-hover". Plasma tries the plain "<state>-hover" before
"<edge>-hover", so without the copies those panels would get the bottom
panel's hover.

Needs Inkscape, to measure where the frames are in the file.
"""

import copy
import math
import shutil
import subprocess
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
UNDERLINE_FILE = REPO / "theme/desktoptheme/protea-dark/widgets/tasks.svg"
BACKGROUND_FILE = REPO / "theme/variants/tasks-background.svg"

# Each style's frames: (tint, line), each a color scheme color (by its
# ColorScheme- class) and an opacity. The line runs along the bottom edge.
STYLES = {
    # A line under each open app; only the focused app is tinted.
    "underline": {
        "normal": (("Text", 0), ("Text", 0.35)),
        "focus": (("ButtonFocus", 0.45), ("ButtonFocus", 0.90)),
        "minimized": (("Text", 0), ("Text", 0.35)),
        "attention": (("NeutralText", 0), ("NeutralText", 0.80)),
        # The hover frames keep the state's line, with a grey tint behind
        # it; only the focused app gets the accent.
        "focus-hover": (("ButtonHover", 0.34), ("ButtonHover", 1)),
        "normal-hover": (("Text", 0.15), ("Text", 0.35)),
        "minimized-hover": (("Text", 0.15), ("Text", 0.35)),
        "attention-hover": (("Text", 0.15), ("NeutralText", 0.80)),
        # Only used if a state above has no hover frame of its own.
        "hover": (("Text", 0.15), ("Text", 0.35)),
    },
    # A tint behind each open app, with no line until the pointer is over it.
    "background": {
        "normal": (("Text", 0.15), ("Text", 0.15)),
        "focus": (("ButtonFocus", 0.45), ("ButtonFocus", 0.45)),
        "minimized": (("Text", 0.15), ("Text", 0.15)),
        "attention": (("NeutralText", 0.35), ("NeutralText", 0.35)),
        "focus-hover": (("ButtonFocus", 0.45), ("ButtonHover", 1)),
        "normal-hover": (("Text", 0.15), ("Text", 0.40)),
        "minimized-hover": (("Text", 0.15), ("Text", 0.40)),
        "attention-hover": (("NeutralText", 0.35), ("NeutralText", 0.80)),
        "hover": (("Text", 0.15), ("Text", 0.40)),
    },
}

LINE = 2   # px, the line's height
CLEAR = 1  # px left clear at each side of a frame

# The frames Breeze ships; the "-hover" ones above are added below them, in
# the same column and spacing.
SHIPPED = ["focus", "normal", "hover", "attention", "minimized"]
EDGES = ["north", "east", "west"]

PIECES = ["topleft", "top", "topright", "left", "center", "right", "bottomleft", "bottom", "bottomright"]

MARK = "protea-task-style"

SVG = "http://www.w3.org/2000/svg"
INDENT = "    "


def q(tag):
    return f"{{{SVG}}}{tag}"


def num(v):
    return f"{v:.4f}".rstrip("0").rstrip(".")


def query(path):
    """Each element's bounding box (x, y, width, height), measured by Inkscape."""
    try:
        out = subprocess.run(["inkscape", "--query-all", str(path)], capture_output=True, text=True, check=True).stdout
    except FileNotFoundError:
        sys.exit("task-frames: needs Inkscape (inkscape) to measure the frames")
    boxes = {}
    for line in out.splitlines():
        parts = line.split(",")
        if len(parts) == 5:
            boxes[parts[0]] = tuple(round(float(v), 3) for v in parts[1:])
    return boxes


def load(path):
    # Plain <style>, not <svg:style>: Plasma finds the stylesheet it puts
    # the theme's colors in by its tag.
    ET.register_namespace("", SVG)
    return ET.parse(path)


def save(tree, path):
    """Write tree to path in the file's own style: "/>" with no space before it."""
    text = ET.tostring(tree.getroot(), encoding="unicode").replace(" />", "/>")
    Path(path).write_text(text + "\n")


def rect(x, y, w, h, color, opacity):
    return ET.Element(q("rect"), {"x": num(x), "y": num(y), "width": num(w), "height": num(h),
                                  "class": f"ColorScheme-{color}", "fill": "currentColor", "opacity": num(opacity)})


def piece(prefix, name, box, tint, line):
    """Piece name of a frame, filling box: the tint, and the line along the bottom row."""
    x, y, w, h = box
    side = name.removeprefix("top").removeprefix("bottom") or name
    if side == "left":
        x, w = x + CLEAR, w - CLEAR
    elif side == "right":
        w -= CLEAR
    bottom = name.startswith("bottom")
    g = ET.Element(q("g"), {"id": f"{prefix}-{name}"})
    # A clear box the piece's full size, so it keeps that size where it's
    # empty (Breeze does the same).
    ET.SubElement(g, q("rect"), {"x": num(box[0]), "y": num(box[1]), "width": num(box[2]), "height": num(box[3]), "fill": "none"})
    tint_h = h - LINE if bottom else h
    if tint[1] and tint_h:
        g.append(rect(x, y, w, tint_h, *tint))
    if bottom and line[1]:
        g.append(rect(x, y + tint_h, w, LINE, *line))
    return g


def place(root, el, old, at=None):
    """Put el where old was, or at index at, or at the end of the file."""
    if old is not None:
        at = list(root).index(old)
        el.tail = old.tail
        root.remove(old)
    elif at is not None:
        el.tail = "\n" + INDENT
    else:
        root[-1].tail = "\n" + INDENT
        el.tail = "\n"
    root.insert(len(root) if at is None else at, el)
    ET.indent(el, space=INDENT, level=1)


def translate(el, dx, dy):
    if dx or dy:
        t = el.get("transform")
        el.set("transform", f"translate({num(dx)},{num(dy)})" + (f" {t}" if t else ""))


def redraw(svg, style):
    states = STYLES[style]
    boxes = query(svg)
    tree = load(svg)
    root = tree.getroot()
    if root.get("viewBox", "").split()[:2] != ["0", "0"]:
        sys.exit(f"{svg}: expected a viewBox at 0,0, got {root.get('viewBox')}")
    by_id = {e.get("id"): e for e in root if e.get("id")}

    # The style's mark, after the stylesheet.
    mark = ET.Element(q("desc"), {"id": MARK})
    mark.text = style
    place(root, mark, by_id.get(MARK), at=list(root).index(by_id["current-color-scheme"]) + 1)

    def frame_box(prefix):
        return {p: boxes[f"{prefix}-{p}"] for p in PIECES}

    # New frames go in rows below Breeze's, spaced like them.
    tops = sorted(boxes[f"{s}-top"][1] for s in SHIPPED)
    pitch = tops[1] - tops[0]
    added = [s for s in states if s not in SHIPPED]
    row = {s: tops[-1] + pitch * (i + 1) for i, s in enumerate(added)}

    normal = frame_box("normal")
    for state, (tint, line) in states.items():
        if state in SHIPPED:
            pieces = frame_box(state)
        else:
            dy = row[state] - normal["top"][1]
            pieces = {p: (x, y + dy, w, h) for p, (x, y, w, h) in normal.items()}
        for p, box in pieces.items():
            place(root, piece(state, p, box, tint, line), by_id.get(f"{state}-{p}"))

    # Top and side panels: a copy of their own hover frame for each new
    # "<state>-hover", in the same row as the bottom panel's.
    for state in added:
        for edge in EDGES:
            hover_top = min(boxes[f"{edge}-hover-{p}"][1] for p in PIECES)
            for p in PIECES:
                el = copy.deepcopy(by_id[f"{edge}-hover-{p}"])
                for e in el.iter():
                    e.attrib.pop("id", None)
                el.set("id", f"{edge}-{state}-{p}")
                translate(el, 0, row[state] - hover_top)
                place(root, el, by_id.get(el.get("id")))
    save(tree, svg)

    # Fit the page around everything, so it's all on the page when editing.
    x, y, w, h = query(svg)[root.get("id")]
    tree = load(svg)
    root = tree.getroot()
    root.set("viewBox", f"0 0 {math.ceil(x + w)} {math.ceil(y + h)}")
    save(tree, svg)


def main():
    if len(sys.argv) not in (1, 3):
        sys.exit(__doc__)
    underline, background = (Path(p) for p in sys.argv[1:]) if len(sys.argv) == 3 else (UNDERLINE_FILE, BACKGROUND_FILE)
    redraw(underline, "underline")
    print(f"redrew the underline task frames in {underline}")
    background.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(underline, background)
    redraw(background, "background")
    print(f"redrew the background task frames in {background}")


if __name__ == "__main__":
    main()
