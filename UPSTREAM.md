# Upstream sources

Protea Dark is forked from KDE's Breeze, tag `v6.8.1`.
Imported with `scripts/import-upstream.sh v6.8.1`.

| Path in this repo | Upstream repo | Upstream path | Commit |
|---|---|---|---|
| `cpp/` | [plasma/breeze](https://invent.kde.org/plasma/breeze) | `/` (without `cursors/`, `wallpapers/`) | `23c317d08b38b160fadd3aeb21a463cb89d12fce` |
| `theme/color-schemes/BreezeDark.colors` | [plasma/breeze](https://invent.kde.org/plasma/breeze) | `colors/BreezeDark.colors` | `23c317d08b38b160fadd3aeb21a463cb89d12fce` |
| `theme/desktoptheme/breeze/` | [plasma/libplasma](https://invent.kde.org/plasma/libplasma) | `src/desktoptheme/breeze/` | `f46c05f2174e8cf35c8d46f4e8cd8fe7d595874e` |
| `theme/desktoptheme/breeze-dark/` | [plasma/libplasma](https://invent.kde.org/plasma/libplasma) | `src/desktoptheme/breeze-dark/` | `f46c05f2174e8cf35c8d46f4e8cd8fe7d595874e` |
| `theme/look-and-feel/org.kde.breezedark/` | [plasma/plasma-workspace](https://invent.kde.org/plasma/plasma-workspace) | `lookandfeel/org.kde.breezedark/` | `fb9b679a99078e68164b05b9cb3f5aaa6020222f` |

## Licenses

- `cpp/`: GPL-2.0-or-later and others; see `cpp/LICENSES/` and the SPDX headers in each file.
- Plasma Style (libplasma): LGPL, per its `metadata.json.cmake`.
- Global Theme (plasma-workspace): GPL-2.0-or-later, per its `metadata.json`.
- Color scheme: see the SPDX headers in the `.colors` file.

License texts for `theme/` are in `theme/LICENSES/`.
