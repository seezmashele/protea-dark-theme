# Protea Dark: project plan

Protea Dark is a KDE Plasma theme from **Protea Desktop**, an organisation that will publish several themes and apps. It is a fork of KDE's Breeze theme at tag `v6.8.1`, targeting Plasma 6.8. Its installer is a terminal app written in Go with Bubble Tea.

Where each upstream file came from is recorded in [UPSTREAM.md](UPSTREAM.md).

## Goals

- **Update-safe core.** The theme you install should keep working through Plasma updates without a rebuild.
- **Opt-in C++.** Breeze's compiled parts (application style and window decoration) are forked too. They are optional, and users build them themselves.
- **Dark only for now.** IDs include `-dark`, so a light variant can be added later without renaming anything.
- **Coexists with Breeze.** Every ID, file name and plugin name differs from Breeze's, so both can be installed side by side.

## Architecture

The theme has two parts.

### 1. Core theme (always installed)

Data files only: nothing is compiled and no root access is needed. It installs per user into `~/.local/share/`.

| Component | ID | Install location | Upstream source |
|---|---|---|---|
| Global Theme | `org.proteadesktop.protea-dark` | `~/.local/share/plasma/look-and-feel/` | plasma-workspace `lookandfeel/org.kde.breezedark` |
| Plasma Style | `protea-dark` | `~/.local/share/plasma/desktoptheme/` | libplasma `src/desktoptheme/breeze` (SVGs) + `breeze-dark` (colors) |
| Color scheme | `ProteaDark` | `~/.local/share/color-schemes/` | breeze `colors/BreezeDark.colors` |

How the core stays update-safe:

- **No QML.** Like Breeze Dark itself, the Global Theme ships only `metadata.json`, `contents/defaults`, `contents/layouts` and `contents/previews`. Plasma loads the parts a theme leaves out (splash, lock screen, logout screen, window switcher) from `org.kde.breeze.desktop`, so they always match the installed Plasma version.
- **Missing SVGs fall back too.** If a Plasma release adds a new SVG element, Plasma uses Breeze's version until Protea adds its own.
- **Defaults use stock Breeze plugins.** The Global Theme's `contents/defaults` select Breeze's application style and window decoration, so the core works without the C++ part. If they selected Protea's plugins and those were not installed, Qt would fall back to its plain Fusion style.

Planned `contents/defaults`:

```ini
[kdeglobals][KDE]
widgetStyle=Breeze

[kdeglobals][General]
ColorScheme=ProteaDark

[kdeglobals][Icons]
Theme=breeze-dark

[plasmarc][Theme]
name=protea-dark

[kcminputrc][Mouse]
cursorTheme=breeze_cursors

[kwinrc][org.kde.kdecoration2]
library=org.kde.breeze
theme=Breeze

[ksplashrc][KSplash]
Theme=org.kde.breeze.desktop
```

Upstream Breeze Dark sets `plasmarc` `name=default`, which is the Breeze style that follows the color scheme. Protea points at its own `protea-dark` instead.

### 2. Optional C++ components (users build them)

A fork of the breeze repo's compiled plugins. Both take their colors from the active color scheme, so their IDs have no `-dark`.

| Component | Protea | Breeze equivalent |
|---|---|---|
| Application style | Qt style key `Protea`, `protea6.so` | `Breeze`, `breeze6.so` |
| Window decoration | `org.proteadesktop.protea` | `org.kde.breeze` |
| Common code | `proteacommon6` (CMake OBJECT library, compiled into each plugin, never installed) | `breezecommon6` |
| Config file | `protearc` | `breezerc` |
| Settings pages | `proteastyleconfig`, `kcm_proteadecoration` | `breezestyleconfig`, `kcm_breezedecoration` |
| Settings app | `protea-settings6` | `breeze-settings6` |
| C++ namespace | `Protea::` | `Breeze::` |

Every installed file has a Protea name: the `.kcfg` schema files (`protea.kcfg`, `protea-decoration.kcfg`), `protea.themerc`, the `protea-settings` icon, the settings pages' `.desktop` files, and the translations (`protea_style_config`, `protea_kwin_deco`). None of them collides with a file already on the system.

What deliberately keeps a Breeze name:

- **Widget properties** that KDE apps set to talk to the style, such as `_breeze_force_frame` and `_breeze_borders_sides`. Renaming them would make Protea ignore those requests.
- **The Global Theme defaults:** they still select Breeze's style and decoration until the C++ is installed.

How the rename was done:

- **Mechanical part:** [scripts/rebrand-cpp.py](scripts/rebrand-cpp.py), which is reusable on a fresh upstream import.
  - Removes the unused upstream pieces.
  - Renames files and text.
  - Moves D-Bus to `org.proteadesktop.Protea.Style`.
  - Strips translated plugin names and keywords, since they name "Breeze" in translation.
  - Keeps translated descriptions that don't mention Breeze.
- **Manual part:** edits to the root `CMakeLists.txt`.
  - Removed the cursors, wallpapers, colors, `misc` and CMake-package sections, KDE's CI switch and the git pre-commit hook.
  - Made Qt5 off by default.
  - Set the version to 0.1.0.

Removed from upstream (neither built nor needed):

- `colors/`
- `misc/`: QtCurve preset and a Breeze → BreezeLight migration tool
- `printing/`
- `cmake/`
- `BreezeConfig.cmake.in`
- KDE's CI files

Build configuration:

- **Options:** defaults are Qt6 only (`BUILD_QT5=OFF`). Qt5 apps keep using Breeze. Pass `-DBUILD_TESTING=OFF` for normal builds.
- **New `WITH_STYLE` option:** upstream can already skip the decoration (`WITH_DECORATIONS`) but always builds the application style. With `WITH_STYLE` added, either component can be built alone.
- **Build dependencies on Arch:** `cmake` and `extra-cmake-modules`. Everything else is already installed as a dependency of Breeze.
- **Packaging:** a PKGBUILD in `packaging/arch/`, built with `makepkg -si`. pacman tracks the files, and `pacman -R` removes them cleanly.

#### Rebuild policy

| Update | Rebuild needed? | Why |
|---|---|---|
| Plasma bugfix (6.8.x) | No | Binary compatibility is kept within a feature series. |
| KDE Frameworks (monthly KF6) | No | KF6 is binary compatible across releases. |
| Qt minor (6.12 → 6.13) | No | Breeze uses only public Qt APIs. Qt will not load a plugin built against a newer Qt than the one installed, so build against the oldest supported Qt. |
| Plasma feature (6.8 → 6.9) | **Yes** | KDecoration has no binary-compatibility promise, and its soname stays `.so.6` across Plasma 6. A mismatched decoration runs inside KWin, so it can crash the session instead of failing cleanly. |
| Plasma 7 / Qt 7 / KF7 | Yes, plus porting | New APIs. |

Breeze also changes in each feature release, so each one means rebasing onto the new upstream tag (see [Upgrading upstream](#upgrading-upstream)).

## Installer

A single Go binary that embeds `theme/` and `cpp/`.

- **Libraries:**
  - Bubble Tea v2.1.0 (`charm.land/bubbletea/v2`)
  - Bubbles v2.2.1
  - Lip Gloss v2.0.6
- **Toolchain:** Go 1.27.
- **Module path:** `proteadesktop.org/protea-theme/installer`. This doesn't need to resolve anywhere while users build from source.
- **Future scope:** it may later grow into a shared installer for all Protea Desktop themes.

### Pages

1. **Welcome.** Shows the detected Plasma version (`plasmashell --version`) and the status of each part: installed or not, which Plasma version the C++ was built for, out of date or current.
2. **Theme.** Installs or uninstalls the core theme into `~/.local/share/`, and optionally applies it with `plasma-apply-lookandfeel`.
3. **Optional components.** Holds the C++ part:
   - A checkbox each for the application style and window decoration, with their current status.
   - The status of the build tools, offering the `pacman -S --needed cmake extra-cmake-modules` command if they are missing.
   - A warning that a rebuild is needed after each Plasma feature update.
   - Actions: **Build & install**, **Rebuild**, **Revert to Breeze**, **Remove**.
   - Builds run through `makepkg -si`. `tea.ExecProcess` hands over the terminal so sudo can prompt for a password.
   - After building, it switches the active style and decoration: `widgetStyle=Protea` in `kdeglobals`, and the decoration `library=` in `kwinrc`, then reconfigures KWin. The exact commands are still to be verified.
   - It records the Plasma version used for the build. If that doesn't match the running Plasma, it offers Rebuild or Revert to Breeze instead of switching.
4. **Progress.** A live build log, with a spinner per step.
5. **Summary.**

`--uninstall` is also available as a flag, for scripting.

## Distribution

- **Now:** source only. Users run the installer, which builds the C++ part on their machine if they choose it.
- **Later, optional:**
  - **AUR package.** AUR helpers rebuild it automatically.
  - **KDE Store listing for the core theme.** The store can't distribute compiled plugins.
- **Not planned:** prebuilt binaries. They would need a new build for every Plasma feature release on every distro.

## Repo layout

```
protea-theme/
├── PLAN.md                     this file
├── UPSTREAM.md                 upstream repos, tag and commit for each imported piece
├── scripts/
│   ├── import-upstream.sh      imports pristine Breeze sources at a given tag
│   └── rebrand-cpp.py          renames an imported Breeze C++ tree to Protea
├── theme/                      core theme
│   ├── color-schemes/
│   ├── desktoptheme/
│   ├── look-and-feel/
│   └── LICENSES/               license texts from libplasma + plasma-workspace
├── cpp/                        forked breeze repo (kstyle, kdecoration, libproteacommon)
├── packaging/arch/             PKGBUILD for the C++ part            (planned)
└── installer/                  Go + Bubble Tea installer            (planned)
```

## Roadmap

| # | Step | Status |
|---|---|---|
| 1 | Install build tools (`cmake`, `extra-cmake-modules`) | Done |
| 2 | Import pristine Breeze Dark from `v6.8.1` | Done |
| 3 | Baseline build of unmodified C++: builds clean, installs the same files as Arch's `breeze` package (minus cursors, wallpapers, translations) | Done |
| 4 | Commit the pristine import (repo owner) | Done |
| 5 | Rename the core theme to Protea: merge the two Plasma Style folders into `protea-dark` with a plain `metadata.json`, rename the color scheme to `ProteaDark`, rename the Global Theme to `org.proteadesktop.protea-dark`, and point its defaults at Protea's color scheme and Plasma Style | Done |
| 6 | Verify the core theme: install it into `~/.local/share/`, check it appears next to Breeze under Global Theme, Plasma Style and Colors, and apply it | Done |
| 7 | Rename the C++ to Protea: all IDs above, the C++ namespace, installed file names and translations; remove unused upstream pieces. Verified: builds clean, no installed file collides with any file on the system, Qt loads the `Protea` style, and the decoration reports ID `org.proteadesktop.protea`. | Done |
| 8 | Add the `WITH_STYLE` CMake option and write the PKGBUILD. Install with `makepkg -si`, verify Breeze and Protea both appear under Application Style and Window Decorations, and test reverting to Breeze. | |
| 9 | Build the Go installer | |
| 10 | Design work: colors, SVGs, style metrics (`kstyle/proteametrics.h`), decoration drawing (corner radius in `kdecoration/protea.h`) | |

Commit after each step.

## Upgrading upstream

When Plasma ships a new feature release:

1. Clone the old and new upstream tags. `scripts/import-upstream.sh` keeps clones in `~/.cache/protea-theme/upstream/<TAG>`.
2. Copy each tag's breeze repo to a scratch folder, and run `scripts/rebrand-cpp.py` on both copies.
3. Diff the renamed old tree against the renamed new tree, and apply that patch to `cpp/`. Resolve any conflicts by hand, and redo the manual `CMakeLists.txt` edits if upstream changed those sections.
4. For `theme/`, diff the old and new upstream files directly, and apply the changes by hand. There are only a few files, and the names differ.

`import-upstream.sh` refuses to run over an existing `cpp/` or `theme/`, so it never overwrites Protea changes.

## Licensing

Protea keeps Breeze's licenses and copyright headers.

- `cpp/`: GPL-2.0-or-later and others (see `cpp/LICENSES/`).
- Plasma Style: LGPL.
- Global Theme: GPL-2.0-or-later.
- Color scheme: per its SPDX headers.

## Open items

- Register `proteadesktop.org` before the first public release. The `org.proteadesktop` IDs imply ownership of that domain.
- Light variant later. IDs are already shaped for it (`protea-light`, `ProteaLight`).
- Replace the Global Theme previews (`contents/previews/`). They are still Breeze Dark screenshots.
- Colors live in two files: the color scheme (`ProteaDark.colors`) and the Plasma Style's own `colors`. Color changes need making in both, or the panels won't match the apps.
- Verify the exact commands the installer uses to switch style and decoration and to reconfigure KWin.

## Project conventions

- **Repo management.** The repo owner manages git and GitHub. Tooling doesn't create repos, commit or push. Work stops at checkpoints so the owner can commit.
- **IDs are permanent once released.** Users' configs store them (for example `LookAndFeelPackage=` in `kdeglobals`), so renaming one later breaks existing installs.
