#!/usr/bin/env python3
"""Redraw the rounded corners of the Plasma Style's panels, popups and tooltips.

Usage: scripts/frame-radius.py RADIUS [SVG ...]

Plasma draws panels, popups and tooltips as frames: nine pieces (four
corners, four edges and a center) that it stretches to any size. This script
redraws the pieces of every frame in each SVG for a corner radius in pixels:
the background, the blur mask KWin uses for the blur behind it, the shadow,
and the header and footer strips inside popups (widgets/plasmoidheading).
Each file keeps its own colors, opacity and hints (margins), and everything
else in it is left alone.

With no SVGs given, it redraws the Plasma Style's panels, popups, tooltips
and popup headers (FRAMES below), including the opaque/ versions used
without compositing. Without compositing there's no transparency to smooth
a curve with, so their corners are drawn in whole pixels, as a staircase
with the outline along it, and the window's shape (the mask) follows the
same pixels. Their radius has to be a whole number of pixels.

The window decoration's radius is set separately (Frame_FrameRadius in
cpp/kdecoration/protea.h, in units of 2 px), and so is the app style's
(Frame_FrameRadius in cpp/kstyle/proteametrics.h); keep all three the same.

Needs Inkscape, to measure where the frames are in each file.
"""

import math
import subprocess
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

STYLE_DIR = Path(__file__).resolve().parent.parent / "theme/desktoptheme/protea-dark"

FRAMES = [
    "dialogs/background.svg",
    "translucent/dialogs/background.svg",
    "solid/dialogs/background.svg",
    "widgets/panel-background.svg",
    "translucent/widgets/panel-background.svg",
    "solid/widgets/panel-background.svg",
    "widgets/tooltip.svg",
    "translucent/widgets/tooltip.svg",
    "solid/widgets/tooltip.svg",
    "widgets/plasmoidheading.svg",
    "opaque/dialogs/background.svg",
    "opaque/widgets/panel-background.svg",
    "opaque/widgets/tooltip.svg",
]

# Breeze's shadow, read off its SVGs: a box the size of the frame, shrunk by
# these insets (so it sits 2 px lower), fading out over SHADOW_FALLOFF px
# through SHADOW_STOPS (position along the fade, opacity).
SHADOW_INSET = {"top": 4, "right": 2, "bottom": 0, "left": 2}
SHADOW_FALLOFF = 10
SHADOW_STOPS = [(0, 0.2), (1 / 3, 0.1), (0.5, 0.05), (7 / 12, 0.025), (1, 0)]

# Breeze fills the part of a shadow piece under the frame with this, nearly
# invisible, so each piece keeps its full size.
PLACEHOLDER = {"style": "opacity:0.00100002;fill:#000000;fill-opacity:0.00392157"}

# Frames, by the prefix of their pieces' ids.
KINDS = {"": "background", "mask-": "mask", "shadow-": "shadow", "header-": "header", "footer-": "footer"}

CORNERS = {"topleft": (-1, -1), "topright": (1, -1), "bottomleft": (-1, 1), "bottomright": (1, 1)}
PIECES = [*CORNERS, "top", "bottom", "left", "right", "center"]

# Attributes that say how a shape is painted, copied from the old pieces.
PAINT = ["class", "style", "fill", "opacity", "fill-opacity", "fill-rule"]

SVG = "http://www.w3.org/2000/svg"
GAP = 4  # between frames in the file, so they're easy to tell apart when editing


def q(tag):
    return f"{{{SVG}}}{tag}"


def num(v):
    return f"{v:.4f}".rstrip("0").rstrip(".")


def query(path):
    """Each element's bounding box (x, y, width, height), measured by Inkscape."""
    try:
        out = subprocess.run(["inkscape", "--query-all", str(path)], capture_output=True, text=True, check=True).stdout
    except FileNotFoundError:
        sys.exit("frame-radius: needs Inkscape (inkscape) to measure the frames")
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


def invisible(e):
    """Whether shape e paints nothing, like the placeholder some pieces start with to keep their size."""
    style = (e.get("style") or "").replace(" ", "")
    return e.get("fill") == "none" or "fill:none" in style


def paint(el):
    """How the first visible shape in el (or el itself) is painted."""
    for e in el.iter():
        if e.tag in (q("rect"), q("path")) and not invisible(e):
            return {k: e.get(k) for k in PAINT if e.get(k) is not None}
    return {}


def shapes(el):
    return [e for e in el.iter() if e.tag in (q("rect"), q("path"))]


def rect(x, y, w, h, attrs):
    return ET.Element(q("rect"), {"x": num(x), "y": num(y), "width": num(w), "height": num(h), **attrs})


def path(d, attrs):
    return ET.Element(q("path"), {"d": d, **attrs})


def piece(name, *children):
    g = ET.Element(q("g"), {"id": name})
    g.extend(children)
    return g


def sweep(cx, cy, p1, p2):
    """SVG's sweep flag for the quarter arc around (cx, cy) from p1 to p2."""
    a1 = math.atan2(p1[1] - cy, p1[0] - cx)
    a2 = math.atan2(p2[1] - cy, p2[0] - cx)
    return 1 if math.remainder(a2 - a1, 2 * math.pi) > 0 else 0


def rounded(ix, iy, sx, sy, r):
    """A quarter disc: the corner at inner corner (ix, iy), facing (sx, sy)."""
    p1, p2 = (ix, iy + sy * r), (ix + sx * r, iy)
    return (f"M {num(ix)},{num(iy)} L {num(p1[0])},{num(p1[1])} "
            f"A {num(r)},{num(r)} 0 0 {sweep(ix, iy, p1, p2)} {num(p2[0])},{num(p2[1])} Z")


def corner_pixels(r):
    """The whole pixels of an r x r corner that lie inside its quarter circle,
    as (x, y) counted from the frame's outer edges."""
    return {(x, y) for x in range(r) for y in range(r) if (r - x - 0.5) ** 2 + (r - y - 0.5) ** 2 <= r * r}


def crisp_corner(ix, iy, sx, sy, r, fill, line):
    """A corner in whole pixels, at inner corner (ix, iy) facing (sx, sy): one
    rect per row of fill, and the line over the outermost pixels."""
    inside = corner_pixels(r)
    edge = {(x, y) for x, y in inside if (x - 1, y) not in inside or (x, y - 1) not in inside}
    ox, oy = ix + sx * r, iy + sy * r  # the outer edges

    def run(a, b, y, attrs):
        """Pixels a to b (exclusive) of row y, counted from the outer edges."""
        left = ox + a if sx < 0 else ox - b
        top = oy + y if sy < 0 else oy - y - 1
        return rect(left, top, b - a, 1, attrs)

    out = []
    for y in range(r):
        xs = sorted(x for x, yy in inside if yy == y)
        out.append(run(xs[0], r, y, fill))
    if line:
        for y in range(r):
            xs = sorted(x for x, yy in edge if yy == y)
            start = xs[0]
            for a, b in zip(xs, xs[1:] + [None]):
                if b != a + 1:
                    out.append(run(start, a + 1, y, line))
                    start = b
    return out


def frame(kind, prefix, box, r, looks, shadow, crisp=False):
    """The nine pieces of a frame whose center is box, as a group. A crisp
    frame, for use without compositing, has whole-pixel corners."""
    x0, y0, w, h = box
    inner = {"topleft": (x0, y0), "topright": (x0 + w, y0), "bottomleft": (x0, y0 + h), "bottomright": (x0 + w, y0 + h)}
    fill, line = looks.get("fill", {}), looks.get("line")
    out = []

    def strip(x, y, ww, hh):
        return [rect(x, y, ww, hh, line)] if line else []

    if kind in ("background", "mask"):
        for name, (sx, sy) in CORNERS.items():
            ix, iy = inner[name]
            if crisp:
                # The mask is the window's shape here, not a blur mask, so
                # it's the same size as the frame.
                out.append(piece(prefix + name, *crisp_corner(ix, iy, sx, sy, int(r), fill, line if kind == "background" else None)))
            elif kind == "background":
                out.append(piece(prefix + name, path(rounded(ix, iy, sx, sy, r), fill)))
            else:
                # The blur mask isn't antialiased, so Breeze makes its corners
                # 1 px smaller than the frame's; the spikes along the inner
                # edges keep the piece r x r.
                m = r - 1
                p1, p2 = (ix, iy + sy * m), (ix + sx * m, iy)
                d = (f"M {num(p2[0])},{num(iy)} L {num(ix + sx * r)},{num(iy)} L {num(ix)},{num(iy)} "
                     f"L {num(ix)},{num(iy + sy * r)} L {num(p1[0])},{num(p1[1])} "
                     f"A {num(m)},{num(m)} 0 0 {sweep(ix, iy, p1, p2)} {num(p2[0])},{num(p2[1])} Z")
                out.append(piece(prefix + name, path(d, fill)))
        out += [
            piece(prefix + "top", rect(x0, y0 - r, w, r, fill), *strip(x0, y0 - r, w, 1)),
            piece(prefix + "bottom", rect(x0, y0 + h, w, r, fill), *strip(x0, y0 + h + r - 1, w, 1)),
            piece(prefix + "left", rect(x0 - r, y0, r, h, fill), *strip(x0 - r, y0, 1, h)),
            piece(prefix + "right", rect(x0 + w, y0, r, h, fill), *strip(x0 + w + r - 1, y0, 1, h)),
        ]

    elif kind in ("header", "footer"):
        # The header's top corners are rounded and a separator runs along its
        # bottom; the footer is the other way up.
        sep = looks["separator"]
        rounded_side = -1 if kind == "header" else 1
        sep_y = y0 + h + r - 1 if kind == "header" else y0 - r
        for name, (sx, sy) in CORNERS.items():
            ix, iy = inner[name]
            if sy == rounded_side:
                out.append(piece(prefix + name, path(rounded(ix, iy, sx, sy, r), fill)))
            else:
                x = min(ix, ix + sx * r)
                out.append(piece(prefix + name, rect(x, min(iy, iy + sy * r), r, r, fill), rect(x, sep_y, r, 1, sep)))
        top_sep = [rect(x0, sep_y, w, 1, sep)] if kind == "footer" else []
        bottom_sep = [rect(x0, sep_y, w, 1, sep)] if kind == "header" else []
        out += [
            piece(prefix + "top", rect(x0, y0 - r, w, r, fill), *top_sep),
            piece(prefix + "bottom", rect(x0, y0 + h, w, r, fill), *bottom_sep),
            piece(prefix + "left", rect(x0 - r, y0, r, h, fill)),
            piece(prefix + "right", rect(x0 + w, y0, r, h, fill)),
        ]

    elif kind == "shadow":
        s = shadow["margins"]
        gradients = shadow["gradients"]
        ins, fall = SHADOW_INSET, SHADOW_FALLOFF

        def linear(name, **coords):
            gradients.append(gradient("linearGradient", name, {k: num(v) for k, v in coords.items()},
                                      [(o, a) for o, a in SHADOW_STOPS]))
            return {"style": f"fill:url(#{name})"}

        left, right, top, bottom = x0 - r, x0 + w + r, y0 - r, y0 + h + r  # the frame's edges
        out += [
            piece(prefix + "top",
                  rect(x0, top - s["top"], w, s["top"], linear("protea-shadow-top", x1=x0, x2=x0, y1=top + ins["top"], y2=top + ins["top"] - fall)),
                  rect(x0, top, w, r, PLACEHOLDER)),
            piece(prefix + "bottom",
                  rect(x0, y0 + h, w, r, PLACEHOLDER),
                  rect(x0, bottom, w, s["bottom"], linear("protea-shadow-bottom", x1=x0, x2=x0, y1=bottom - ins["bottom"], y2=bottom - ins["bottom"] + fall))),
            piece(prefix + "left",
                  rect(left - s["left"], y0, s["left"], h, linear("protea-shadow-left", y1=y0, y2=y0, x1=left + ins["left"], x2=left + ins["left"] - fall)),
                  rect(left, y0, r, h, PLACEHOLDER)),
            piece(prefix + "right",
                  rect(x0 + w, y0, r, h, PLACEHOLDER),
                  rect(right, y0, s["right"], h, linear("protea-shadow-right", y1=y0, y2=y0, x1=right - ins["right"], x2=right - ins["right"] + fall))),
        ]
        # The corners fade around the shadow box's own rounded corner: the
        # frame's, shrunk by the insets and moved with them.
        shrink = min((ins["left"] + ins["right"]) / 2, (ins["top"] + ins["bottom"]) / 2)
        shift = ((ins["left"] - ins["right"]) / 2, (ins["top"] - ins["bottom"]) / 2)
        rv = max(r - shrink, 0)
        stops = [(0, SHADOW_STOPS[0][1])] + [((rv + o * fall) / (rv + fall), a) for o, a in SHADOW_STOPS]
        for name, (sx, sy) in CORNERS.items():
            ix, iy = inner[name]
            ex = r + s["left" if sx < 0 else "right"]
            ey = r + s["top" if sy < 0 else "bottom"]
            p1, p2 = (ix, iy + sy * r), (ix + sx * r, iy)
            d = (f"M {num(p1[0])},{num(p1[1])} A {num(r)},{num(r)} 0 0 {sweep(ix, iy, p1, p2)} {num(p2[0])},{num(p2[1])} "
                 f"L {num(ix + sx * ex)},{num(iy)} L {num(ix + sx * ex)},{num(iy + sy * ey)} L {num(ix)},{num(iy + sy * ey)} Z")
            gid = f"protea-shadow-{name}"
            cx, cy = ix + shift[0], iy + shift[1]
            gradients.append(gradient("radialGradient", gid, {"cx": num(cx), "cy": num(cy), "fx": num(cx), "fy": num(cy), "r": num(rv + fall)}, stops))
            out.append(piece(prefix + name, path(rounded(ix, iy, sx, sy, r), PLACEHOLDER), path(d, {"style": f"fill:url(#{gid})"})))

    center_look = PLACEHOLDER if kind == "shadow" else fill
    out.append(piece(prefix + "center", rect(x0, y0, w, h, center_look)))
    g = ET.Element(q("g"), {"id": f"protea-{kind}-frame"})
    g.extend(out)
    return g


def gradient(tag, gid, attrs, stops):
    g = ET.Element(q(tag), {"id": gid, "gradientUnits": "userSpaceOnUse", **attrs})
    for offset, opacity in stops:
        ET.SubElement(g, q("stop"), {"offset": num(offset), "style": f"stop-color:#000000;stop-opacity:{num(opacity)}"})
    return g


def append(parent, el, level):
    """Add el as parent's last child, indented to match the file."""
    ET.indent(el, space="  ", level=level)
    if len(parent):
        parent[-1].tail = "\n" + "  " * level
    else:
        parent.text = "\n" + "  " * level
    el.tail = "\n" + "  " * (level - 1)
    parent.append(el)


def translate(el, dx, dy):
    if dx or dy:
        t = el.get("transform")
        el.set("transform", f"translate({num(dx)},{num(dy)})" + (f" {t}" if t else ""))


def remove_unused_gradients(root, parents):
    """Drop gradients nothing refers to any more, like the old shadow's."""
    while True:
        text = ET.tostring(root, encoding="unicode")
        unused = [e for e in root.iter() if e.tag in (q("linearGradient"), q("radialGradient"))
                  and f"#{e.get('id')})" not in text and f'"#{e.get("id")}"' not in text]
        if not unused:
            return
        for e in unused:
            parents[e].remove(e)


def redraw(svg, r):
    crisp = "opaque" in svg.parts
    if crisp and r != int(r):
        sys.exit(f"{svg}: used without compositing, so it needs a whole number of pixels, not {num(r)}")
    boxes = query(svg)
    tree = load(svg)
    root = tree.getroot()
    parents = {c: p for p in root.iter() for c in p}
    by_id = {e.get("id"): e for e in root.iter() if e.get("id")}

    vb = [float(v) for v in root.get("viewBox", "0 0 0 0").split()]
    if vb[:2] != [0, 0] or (root.get("width") and abs(float(root.get("width")) - vb[2]) > 0.01):
        sys.exit(f"{svg}: expected a viewBox at 0,0 in px, got {root.get('viewBox')} at width {root.get('width')}")

    frames = []
    for prefix, kind in KINDS.items():
        if prefix + "center" not in boxes:
            continue
        x, y, w, h = boxes[prefix + "center"]
        looks = {"fill": paint(by_id[prefix + "center"])}
        if kind == "background":
            top = shapes(by_id["top"])
            looks["line"] = {k: v for k, v in paint(top[1]).items()} if len(top) > 1 else None
        elif kind == "header":
            looks["separator"] = paint(shapes(by_id["header-bottom"])[1])
        elif kind == "footer":
            looks["separator"] = paint(shapes(by_id["footer-top"])[1])
        elif kind == "shadow":
            looks["fill"] = {}
        frames.append({"prefix": prefix, "kind": kind, "box": [round(x), round(y), round(w), round(h)], "looks": looks})

    margins = {side: boxes[f"shadow-hint-{side}-margin"][3 if side in ("top", "bottom") else 2]
               for side in ("top", "bottom", "left", "right")} if any(f["kind"] == "shadow" for f in frames) else {}

    # Lay the frames out side by side, moving a frame (and its hints) right
    # if it would now overlap the one before it.
    right_edge = -math.inf
    for f in sorted(frames, key=lambda f: f["box"][0]):
        x, _, w, _ = f["box"]
        reach = r + (max(margins["left"], margins["right"]) if f["kind"] == "shadow" else 0)
        dx = max(0, right_edge + GAP - (x - reach))
        f["box"][0] += dx
        right_edge = x + dx + w + reach
        if dx:
            for hid, el in by_id.items():
                if hid.startswith(f["prefix"] + "hint-"):
                    translate(el, dx, 0)

    for f in frames:
        for name in [*(f["prefix"] + p for p in PIECES), f"protea-{f['kind']}-frame"]:
            el = by_id.get(name)
            if el is not None:
                parents[el].remove(el)
    remove_unused_gradients(root, parents)

    defs = root.find(q("defs"))
    if defs is None:
        defs = ET.Element(q("defs"))
        root.insert(0, defs)
    for f in frames:
        gradients = []
        append(root, frame(f["kind"], f["prefix"], f["box"], r, f["looks"], {"margins": margins, "gradients": gradients}, crisp), 1)
        for g in gradients:
            append(defs, g, 2)
    save(tree, svg)

    # Move everything right and down if it reaches past the top or left, and
    # fit the page around it.
    rx, ry, rw, rh = query(svg)[root.get("id")]
    dx, dy = max(0, -math.floor(rx)), max(0, -math.floor(ry))
    tree = load(svg)
    root = tree.getroot()
    for el in root:
        if isinstance(el.tag, str) and el.tag not in (q("defs"), q("metadata"), q("style")) and not el.tag.endswith("namedview"):
            translate(el, dx, dy)
    width, height = math.ceil(max(rx + rw + dx, 0)), math.ceil(max(ry + rh + dy, 0))
    root.set("viewBox", f"0 0 {width} {height}")
    if root.get("width"):
        root.set("width", str(width))
        root.set("height", str(height))
    save(tree, svg)


def main():
    if len(sys.argv) < 2:
        sys.exit(__doc__)
    r = float(sys.argv[1])
    if r < 2:
        sys.exit("frame-radius: the radius has to be at least 2 px")
    files = [Path(p) for p in sys.argv[2:]] or [STYLE_DIR / f for f in FRAMES]
    for svg in files:
        redraw(svg, r)
        print(f"redrew {svg} with {num(r)} px corners")


if __name__ == "__main__":
    main()
