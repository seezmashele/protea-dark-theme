#!/usr/bin/env python3
"""Redraw the controls inside Plasma's widget popups.

Usage: scripts/widget-frames.py

Plasma draws text fields, tab bars and separator lines in widgets from the
Plasma Style's SVGs. This script redraws them from the values below:

  widgets/lineedit.svg  every text field, search bars included: rounded
                        corners (FIELD_RADIUS) with a light see-through fill
                        and no outline (FIELD), an outline on hover and
                        focus, and the focus ring 2 px outside
  widgets/tabbar.svg    the selected tab, on all four sides: nothing,
                        instead of Breeze's accent line along its edge
  theme/variants/tabbar-underline.svg
                        the same with an accent line along the bottom of the
                        selected tab, its full width (UNDERLINE); the
                        installer puts it in place of tabbar.svg when chosen
  widgets/button.svg    buttons and flat buttons under the pointer: the
                        accent at HOVER_OPACITY, with no outline
  widgets/viewitem.svg  list items under the pointer, like the launcher's
                        apps: the accent at HOVER_OPACITY, with no outline
  widgets/line.svg      the lines between sections, made transparent
  widgets/plasmoidheading.svg
                        popup headers and footers, background and line, made
                        transparent

Each frame keeps its hint margins, so the text, icons and tabs inside don't
move, and each hidden piece keeps its size, so nothing shifts where it was.
The two tab files are marked with their style (<desc id="protea-tab-style">),
which is how the installer tells which one is installed.
scripts/frame-radius.py redraws plasmoidheading.svg's corners and keeps
them transparent.

Needs Inkscape, to measure where the frames are in the files.
"""

import math
import shutil
import subprocess
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
WIDGETS = REPO / "theme/desktoptheme/protea-dark/widgets"
TAB_UNDERLINE_FILE = REPO / "theme/variants/tabbar-underline.svg"

FIELD_RADIUS = 8

# The text field's frames, each a list of layers: a fill from an inset
# ("fill", inset, look), or a ring between two insets ("ring", inset, inset,
# look). A look is a color scheme color (by its ColorScheme- class) and an
# opacity. Insets count from the frame's outer edge.
FIELD = {
    "base": [("fill", 0, ("Text", 0.10))],
    "hover": [("ring", 0, 1, ("ViewHover", 0.50))],
    "focus": [("ring", 0, 1, ("ViewFocus", 1))],
    # 2 px outside the field (its hint margins): a faint halo, then the
    # outline along the field's edge.
    "focusframe": [("ring", 0, 2, ("ViewFocus", 0.33)), ("ring", 2, 3, ("ViewFocus", 1))],
}

# The selected tab. By default it isn't marked: its frame is empty, keeping
# its size and hint margins, which are the tab's padding. The installer's
# other choice draws a line along its bottom, the tab's full width. Nothing
# on a tab is rounded, so the line runs square to both ends.
UNDERLINE = {"height": 2, "look": ("ButtonFocus", 0.50)}
TAB_EDGES = ["north", "south", "east", "west"]

# Plasma's buttons and list highlights keep Breeze's corners for now.
CONTROL_RADIUS = 5

# Buttons, flat buttons and list items under the pointer: the accent at this
# opacity, with no outline.
HOVER_OPACITY = 0.20

BUTTON_HOVER = {
    # A raised button's hover is drawn over its resting frame, so an opaque
    # ring in the button's color covers the resting outline first.
    "hover": [("ring", 0, 1, ("ButtonBackground", 1)), ("fill", 0, ("ButtonHover", HOVER_OPACITY))],
    "toolbutton-hover": [("fill", 0, ("ButtonHover", HOVER_OPACITY))],
}
VIEWITEM_HOVER = [("fill", 0, ("ButtonFocus", HOVER_OPACITY))]

CORNERS = {"topleft": (-1, -1), "topright": (1, -1), "bottomleft": (-1, 1), "bottomright": (1, 1)}
PIECES = [*CORNERS, "top", "bottom", "left", "right", "center"]

SVG = "http://www.w3.org/2000/svg"


def q(tag):
    return f"{{{SVG}}}{tag}"


def num(v):
    return f"{v:.4f}".rstrip("0").rstrip(".")


def query(path):
    """Each element's bounding box (x, y, width, height), measured by Inkscape."""
    try:
        out = subprocess.run(["inkscape", "--query-all", str(path)], capture_output=True, text=True, check=True).stdout
    except FileNotFoundError:
        sys.exit("widget-frames: needs Inkscape (inkscape) to measure the frames")
    boxes = {}
    for line in out.splitlines():
        parts = line.split(",")
        if len(parts) == 5:
            boxes[parts[0]] = tuple(float(v) for v in parts[1:])
    return boxes


def load(path):
    for _, (prefix, uri) in ET.iterparse(path, events=["start-ns"]):
        if uri != SVG:
            ET.register_namespace(prefix, uri)
    # Plain <style>, not <svg:style>: Plasma finds the stylesheet it puts
    # the theme's colors in by its tag.
    ET.register_namespace("", SVG)
    return ET.parse(path, parser=ET.XMLParser(target=ET.TreeBuilder(insert_comments=True)))


def save(tree, path):
    """Write tree to path, keeping the file's XML declaration and comments before <svg>."""
    text = Path(path).read_text()
    prolog = text[:text.index("<svg")]
    Path(path).write_text(prolog + ET.tostring(tree.getroot(), encoding="unicode") + "\n")


def look(attrs):
    color, opacity = attrs
    return {"class": f"ColorScheme-{color}", "fill": "currentColor", "opacity": num(opacity)}


def rect(x, y, w, h, attrs):
    return ET.Element(q("rect"), {"x": num(x), "y": num(y), "width": num(w), "height": num(h), **attrs})


def path(d, attrs):
    return ET.Element(q("path"), {"d": d, **attrs})


def sweep(cx, cy, p1, p2):
    """SVG's sweep flag for the quarter arc around (cx, cy) from p1 to p2."""
    a1 = math.atan2(p1[1] - cy, p1[0] - cx)
    a2 = math.atan2(p2[1] - cy, p2[0] - cx)
    return 1 if math.remainder(a2 - a1, 2 * math.pi) > 0 else 0


def corner(ix, iy, sx, sy, r, layer):
    """A layer's part of the corner with inner corner (ix, iy), facing (sx, sy), in a frame of radius r."""
    def arc_points(radius):
        return (ix + sx * radius, iy), (ix, iy + sy * radius)

    if layer[0] == "fill":
        _, a, attrs = layer
        ra = r - a
        p1, p2 = arc_points(ra)
        d = (f"M {num(ix)},{num(iy)} L {num(p1[0])},{num(p1[1])} "
             f"A {num(ra)},{num(ra)} 0 0 {sweep(ix, iy, p1, p2)} {num(p2[0])},{num(p2[1])} Z")
    else:
        _, a, b, attrs = layer
        ra, rb = r - a, r - b
        p1, p2 = arc_points(ra)
        q1, q2 = arc_points(rb)
        d = (f"M {num(p1[0])},{num(p1[1])} A {num(ra)},{num(ra)} 0 0 {sweep(ix, iy, p1, p2)} {num(p2[0])},{num(p2[1])} "
             f"L {num(q2[0])},{num(q2[1])} A {num(rb)},{num(rb)} 0 0 {sweep(ix, iy, q2, q1)} {num(q1[0])},{num(q1[1])} Z")
    return path(d, look(attrs))


def frame(prefix, box, r, layers):
    """The nine pieces of a frame filling box, with corners of radius r, drawn with layers."""
    x0, y0, w, h = box
    x1, y1 = x0 + w, y0 + h
    inner = {"topleft": (x0 + r, y0 + r), "topright": (x1 - r, y0 + r), "bottomleft": (x0 + r, y1 - r), "bottomright": (x1 - r, y1 - r)}
    sides = {  # piece box, and where the frame's outer edge is: (side, edge)
        "top": ((x0 + r, y0, w - 2 * r, r), "top"),
        "bottom": ((x0 + r, y1 - r, w - 2 * r, r), "bottom"),
        "left": ((x0, y0 + r, r, h - 2 * r), "left"),
        "right": ((x1 - r, y0 + r, r, h - 2 * r), "right"),
    }
    pieces = []
    for name in PIECES:
        if name in CORNERS:
            sx, sy = CORNERS[name]
            ix, iy = inner[name]
            pbox = (min(ix, ix + sx * r), min(iy, iy + sy * r), r, r)
            shapes = [corner(ix, iy, sx, sy, r, layer) for layer in layers]
        elif name == "center":
            pbox = (x0 + r, y0 + r, w - 2 * r, h - 2 * r)
            shapes = [rect(*pbox, look(layer[2])) for layer in layers if layer[0] == "fill"]
        else:
            pbox, edge = sides[name]
            px, py, pw, ph = pbox
            shapes = []
            for layer in layers:
                a, b, attrs = (layer[1], r, layer[2]) if layer[0] == "fill" else layer[1:]
                if edge == "top":
                    shapes.append(rect(px, y0 + a, pw, b - a, look(attrs)))
                elif edge == "bottom":
                    shapes.append(rect(px, y1 - b, pw, b - a, look(attrs)))
                elif edge == "left":
                    shapes.append(rect(x0 + a, py, b - a, ph, look(attrs)))
                else:
                    shapes.append(rect(x1 - b, py, b - a, ph, look(attrs)))
        g = ET.Element(q("g"), {"id": f"{prefix}-{name}"})
        # A clear box the piece's full size, so it keeps that size where it's
        # empty.
        g.append(rect(*pbox, {"fill": "none"}))
        g.extend(shapes)
        pieces.append(g)
    return pieces


def rounded(r, layers):
    """A frame builder: corners of radius r, drawn with layers."""
    return lambda prefix, box: frame(prefix, box, r, layers)


def tab(line):
    """A frame builder for the selected tab: square, with 1 px corner pieces,
    and if line is given ({"height", "look"}), a line along the bottom, the
    full width of the tab. Otherwise it's empty."""
    def build(prefix, box):
        x0, y0, w, h = box
        x1, y1 = x0 + w, y0 + h
        c = 1  # corner pieces
        t = line["height"] if line else c
        boxes = {
            "topleft": (x0, y0, c, t), "top": (x0 + c, y0, w - 2 * c, t), "topright": (x1 - c, y0, c, t),
            "left": (x0, y0 + t, c, h - 2 * t), "center": (x0 + c, y0 + t, w - 2 * c, h - 2 * t), "right": (x1 - c, y0 + t, c, h - 2 * t),
            "bottomleft": (x0, y1 - t, c, t), "bottom": (x0 + c, y1 - t, w - 2 * c, t), "bottomright": (x1 - c, y1 - t, c, t),
        }
        pieces = []
        for name in PIECES:
            g = ET.Element(q("g"), {"id": f"{prefix}-{name}"})
            g.append(rect(*boxes[name], {"fill": "none"}))
            if line and name.startswith("bottom"):
                g.append(rect(*boxes[name], look(line["look"])))
            pieces.append(g)
        return pieces
    return build


def append(root, el):
    """Add el as root's last child, indented like the rest."""
    ET.indent(el, space="  ", level=1)
    if len(root):
        root[-1].tail = "\n  "
    el.tail = "\n"
    root.append(el)


def redraw(svg, frames):
    """Replace each frame (prefix, builder) in svg, keeping its outer box and hints."""
    boxes = query(svg)
    tree = load(svg)
    root = tree.getroot()
    if root.get("viewBox") and root.get("viewBox").split()[:2] != ["0", "0"]:
        sys.exit(f"{svg}: expected a viewBox at 0,0, got {root.get('viewBox')}")
    parents = {c: p for p in root.iter() for c in p}
    by_id = {e.get("id"): e for e in root.iter() if e.get("id")}
    for prefix, build in frames:
        ids = [f"{prefix}-{p}" for p in PIECES]
        x0 = min(boxes[i][0] for i in ids)
        y0 = min(boxes[i][1] for i in ids)
        x1 = max(boxes[i][0] + boxes[i][2] for i in ids)
        y1 = max(boxes[i][1] + boxes[i][3] for i in ids)
        # Whole pixels, so the frame's edges stay sharp.
        box = tuple(round(v) for v in (x0, y0, x1 - x0, y1 - y0))
        for i in ids:
            parents[by_id[i]].remove(by_id[i])
        g = ET.Element(q("g"), {"id": f"protea-{prefix}-frame"})
        g.extend(build(prefix, box))
        old = by_id.get(f"protea-{prefix}-frame")
        if old is not None:
            parents[old].remove(old)
        append(root, g)
    save(tree, svg)


def mark(svg, mark_id, style):
    """Mark svg with its style in a <desc> element, first in the file."""
    tree = load(svg)
    root = tree.getroot()
    old = next((e for e in root.iter() if e.get("id") == mark_id), None)
    if old is not None:
        old.text = style
    else:
        desc = ET.Element(q("desc"), {"id": mark_id})
        desc.text = style
        desc.tail = root.text
        root.insert(0, desc)
    save(tree, svg)


def hide(svg, ids, only_class=None):
    """Make the shapes in each element of ids transparent (only those of only_class, if given)."""
    tree = load(svg)
    root = tree.getroot()
    by_id = {e.get("id"): e for e in root.iter() if e.get("id")}
    for i in ids:
        for e in by_id[i].iter():
            if e.tag in (q("rect"), q("path")) and (only_class is None or e.get("class") == only_class):
                e.set("opacity", "0")
    save(tree, svg)


def main():
    if len(sys.argv) > 1:
        sys.exit(__doc__)
    # The focus ring sits 2 px outside the field, so its corners are 2 px rounder.
    redraw(WIDGETS / "lineedit.svg",
           [(prefix, rounded(FIELD_RADIUS + (2 if prefix == "focusframe" else 0), layers)) for prefix, layers in FIELD.items()])
    redraw(WIDGETS / "tabbar.svg", [(f"{edge}-active-tab", tab(None)) for edge in TAB_EDGES])
    mark(WIDGETS / "tabbar.svg", "protea-tab-style", "none")
    TAB_UNDERLINE_FILE.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(WIDGETS / "tabbar.svg", TAB_UNDERLINE_FILE)
    redraw(TAB_UNDERLINE_FILE, [(f"{edge}-active-tab", tab(UNDERLINE)) for edge in TAB_EDGES])
    mark(TAB_UNDERLINE_FILE, "protea-tab-style", "underline")
    redraw(WIDGETS / "button.svg", [(prefix, rounded(CONTROL_RADIUS, layers)) for prefix, layers in BUTTON_HOVER.items()])
    redraw(WIDGETS / "viewitem.svg", [("hover", rounded(CONTROL_RADIUS, VIEWITEM_HOVER))])
    hide(WIDGETS / "line.svg", ["horizontal-line", "vertical-line"])
    hide(WIDGETS / "plasmoidheading.svg", [f"{kind}-{p}" for kind in ("header", "footer") for p in PIECES])
    print("redrew the text fields, selected tabs and hover highlights, and hid the separator lines and popup headers and footers")


if __name__ == "__main__":
    main()
