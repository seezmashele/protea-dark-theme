#!/usr/bin/env python3
"""Rename an imported Breeze C++ tree (cpp/) to Protea.

Usage: scripts/rebrand-cpp.py CPP_DIR

This is the mechanical part of the rename, kept as a script so it can be
re-run on a fresh upstream import when rebasing onto a new Plasma release.
Structural CMake edits (removed options and subdirectories) are done by hand
afterwards; see PLAN.md.
"""

import json
import re
import shutil
import sys
from pathlib import Path

# Upstream pieces Protea doesn't use.
REMOVE = [
    "colors",                 # the core theme ships the color scheme
    "misc",                   # QtCurve preset and Breeze->BreezeLight migration tool
    "printing",               # CUPS test page, never built
    "cmake",                  # GtkUpdateIconCache, only used by the cursors
    "BreezeConfig.cmake.in",  # CMake package for projects that build against Breeze
    ".gitlab-ci.yml",
    ".kde-ci.yml",
    ".git-blame-ignore-revs",
]

# Left exactly as upstream: license texts, credits, and the README,
# which is rewritten by hand.
SKIP = {"LICENSES", "AUTHORS", "README.md"}
BINARY = {".svgz", ".png", ".pdf"}

# Widget properties that KDE apps set to talk to the style
# (e.g. Dolphin sets _breeze_force_frame). Renaming them would make
# Protea ignore those requests, so they keep their Breeze names.
KEEP = re.compile(r'(?<=")(?:_breeze_[a-z_]+|breeze_has_toolsarea_palette)(?=")')

# Order matters: specific IDs before the generic rename.
REPLACE = [
    ("org.kde.Breeze.Style", "org.proteadesktop.Protea.Style"),  # D-Bus interface
    ("org.kde.breeze", "org.proteadesktop.protea"),              # decoration plugin ID
    ("BREEZE", "PROTEA"),
    ("Breeze", "Protea"),
    ("breeze", "protea"),
]

# Translated plugin metadata that names Breeze in translation ("Brisa",
# "نسيم", ...) and so can't be renamed by text replacement. Dropped;
# translated descriptions that don't mention Breeze are kept.
STRIP_JSON = {
    "kdecoration/breeze.json": ("Name[", "Description["),
    "kdecoration/config/kcm_breezedecoration.json": ("Name[", "X-KDE-Keywords["),
    "kstyle/config/breezestyleconfig.json": ("Name[", "X-KDE-Keywords["),
}
STRIP_INI = {"kstyle/breeze.themerc": ("Name[",)}


def rename_text(text: str) -> str:
    kept = []

    def hold(match):
        kept.append(match.group(0))
        return f"\0{len(kept) - 1}\0"

    text = KEEP.sub(hold, text)
    for old, new in REPLACE:
        text = text.replace(old, new)
    return re.sub(r"\0(\d+)\0", lambda m: kept[int(m.group(1))], text)


def rename_path(name: str) -> str:
    return name.replace("Breeze", "Protea").replace("breeze", "protea")


def strip_json(path: Path, prefixes):
    data = json.loads(path.read_text(encoding="utf-8"))
    plugin = data["KPlugin"]
    for target in (plugin, data):
        for key in [k for k in target if k.startswith(prefixes)]:
            del target[key]
    path.write_text(json.dumps(data, indent=4, ensure_ascii=False) + "\n", encoding="utf-8")


def strip_ini(path: Path, prefixes):
    lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
    path.write_text("".join(l for l in lines if not l.startswith(prefixes)), encoding="utf-8")


def main(root: Path):
    if not (root / "kstyle" / "breezestyle.cpp").exists():
        sys.exit(f"{root} doesn't look like an un-renamed Breeze tree")

    for name in REMOVE:
        p = root / name
        if p.is_dir():
            shutil.rmtree(p)
        elif p.exists():
            p.unlink()

    for rel, prefixes in STRIP_JSON.items():
        strip_json(root / rel, prefixes)
    for rel, prefixes in STRIP_INI.items():
        strip_ini(root / rel, prefixes)

    for path in root.rglob("*"):
        if not path.is_file() or path.suffix in BINARY:
            continue
        if path.relative_to(root).parts[0] in SKIP:
            continue
        text = path.read_text(encoding="utf-8")
        new = rename_text(text)
        if new != text:
            path.write_text(new, encoding="utf-8")

    # Deepest paths first, so directories are renamed after their contents.
    for path in sorted(root.rglob("*"), key=lambda p: len(p.parts), reverse=True):
        if path.relative_to(root).parts[0] in SKIP:
            continue
        new_name = rename_path(path.name)
        if new_name != path.name:
            path.rename(path.with_name(new_name))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(Path(sys.argv[1]).resolve())
