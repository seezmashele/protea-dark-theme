#!/usr/bin/env bash
# Import pristine Breeze Dark sources from KDE into this repo.
#
# Usage: scripts/import-upstream.sh [TAG]   (default: v6.8.1)
#
# Upstream clones are cached in $PROTEA_UPSTREAM_DIR
# (default: ~/.cache/protea-theme/upstream/<TAG>).
# Refuses to run if cpp/ or theme/ already exist, so it never
# overwrites Protea changes.

set -euo pipefail

TAG="${1:-v6.8.1}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE="${PROTEA_UPSTREAM_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/protea-theme/upstream/$TAG}"

die() { echo "error: $*" >&2; exit 1; }

for d in cpp theme UPSTREAM.md; do
    [[ -e "$ROOT/$d" ]] && die "$ROOT/$d already exists; refusing to overwrite"
done

# Clone (or reuse) a shallow checkout of a KDE Plasma repo at $TAG.
fetch() {
    local repo=$1 dir="$CACHE/$1"
    if [[ ! -d "$dir/.git" ]]; then
        echo "cloning $repo $TAG"
        git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$TAG" \
            "https://invent.kde.org/plasma/$repo.git" "$dir"
    fi
    local head tagged
    head=$(git -C "$dir" rev-parse HEAD)
    tagged=$(git -C "$dir" rev-parse "$TAG^{commit}" 2>/dev/null) || die "$dir has no tag $TAG"
    [[ "$head" == "$tagged" ]] || die "$dir is not checked out at $TAG"
}

commit() { git -C "$CACHE/$1" rev-parse HEAD; }

# Copy a directory tree, skipping the given top-level entries.
copy_tree() {
    local src=$1 dest=$2; shift 2
    local excludes=(--exclude=./.git)
    for e in "$@"; do excludes+=("--exclude=./$e"); done
    mkdir -p "$dest"
    tar -C "$src" "${excludes[@]}" -cf - . | tar -C "$dest" -xf -
}

mkdir -p "$CACHE"
for repo in breeze libplasma plasma-workspace; do fetch "$repo"; done

BREEZE="$CACHE/breeze"
LIBPLASMA="$CACHE/libplasma"
WORKSPACE="$CACHE/plasma-workspace"

# C++: the whole breeze repo, minus cursors and wallpapers.
copy_tree "$BREEZE" "$ROOT/cpp" cursors wallpapers

# Core theme: color scheme, Plasma Style, Global Theme.
mkdir -p "$ROOT/theme/color-schemes"
cp "$BREEZE/colors/BreezeDark.colors" "$ROOT/theme/color-schemes/"
copy_tree "$LIBPLASMA/src/desktoptheme/breeze" "$ROOT/theme/desktoptheme/breeze"
copy_tree "$LIBPLASMA/src/desktoptheme/breeze-dark" "$ROOT/theme/desktoptheme/breeze-dark"
copy_tree "$WORKSPACE/lookandfeel/org.kde.breezedark" "$ROOT/theme/look-and-feel/org.kde.breezedark"

# License texts for the core theme (cpp/ keeps breeze's own LICENSES/).
# Both repos ship most of the same licenses (differing only in line
# wrapping), so take libplasma's and add the ones only plasma-workspace has.
mkdir -p "$ROOT/theme/LICENSES"
cp "$LIBPLASMA"/LICENSES/* "$ROOT/theme/LICENSES/"
for f in "$WORKSPACE"/LICENSES/*; do
    [[ -e "$ROOT/theme/LICENSES/${f##*/}" ]] || cp "$f" "$ROOT/theme/LICENSES/"
done

cat > "$ROOT/UPSTREAM.md" <<EOF
# Upstream sources

Protea Dark is forked from KDE's Breeze, tag \`$TAG\`.
Imported with \`scripts/import-upstream.sh $TAG\`.

| Path in this repo | Upstream repo | Upstream path | Commit |
|---|---|---|---|
| \`cpp/\` | [plasma/breeze](https://invent.kde.org/plasma/breeze) | \`/\` (without \`cursors/\`, \`wallpapers/\`) | \`$(commit breeze)\` |
| \`theme/color-schemes/BreezeDark.colors\` | [plasma/breeze](https://invent.kde.org/plasma/breeze) | \`colors/BreezeDark.colors\` | \`$(commit breeze)\` |
| \`theme/desktoptheme/breeze/\` | [plasma/libplasma](https://invent.kde.org/plasma/libplasma) | \`src/desktoptheme/breeze/\` | \`$(commit libplasma)\` |
| \`theme/desktoptheme/breeze-dark/\` | [plasma/libplasma](https://invent.kde.org/plasma/libplasma) | \`src/desktoptheme/breeze-dark/\` | \`$(commit libplasma)\` |
| \`theme/look-and-feel/org.kde.breezedark/\` | [plasma/plasma-workspace](https://invent.kde.org/plasma/plasma-workspace) | \`lookandfeel/org.kde.breezedark/\` | \`$(commit plasma-workspace)\` |

## Licenses

- \`cpp/\`: GPL-2.0-or-later and others; see \`cpp/LICENSES/\` and the SPDX headers in each file.
- Plasma Style (libplasma): LGPL, per its \`metadata.json.cmake\`.
- Global Theme (plasma-workspace): GPL-2.0-or-later, per its \`metadata.json\`.
- Color scheme: see the SPDX headers in the \`.colors\` file.

License texts for \`theme/\` are in \`theme/LICENSES/\`.
EOF

echo "imported $TAG into $ROOT"
