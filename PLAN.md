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
- **Component options:** `WITH_STYLE` (added by Protea) and `WITH_DECORATIONS` (upstream). Both default to ON, and either can be turned off. Turning both off stops with an error.
- **Build record:** every build installs `/usr/share/protea/build-info`. It lists the Protea, Qt and KDecoration versions it was compiled against, and which components it built. The installer reads it to detect when a Plasma or Qt update needs a rebuild.
- **Build dependencies on Arch:** `cmake` and `extra-cmake-modules`. Everything else is already installed as a dependency of Breeze.
- **Packaging:** [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) builds package `protea` straight from `cpp/`.
  - Build with `makepkg -C`; build and install with `makepkg -Ci`.
  - `PROTEA_WITH_STYLE=OFF` or `PROTEA_WITH_DECORATIONS=OFF` builds a single component.
  - The version comes from `PROJECT_VERSION` in `cpp/CMakeLists.txt`.
  - No debug package is built.
  - pacman tracks the files, and `pacman -R protea` removes them cleanly.
  - **Version pin.** When the window decoration is built, the package depends on `kdecoration` and `kwin` being within the Plasma feature release it was built against: `>=6.8` and `<6.8.80` for a 6.8 build. That allows 6.8.x bugfix releases and blocks 6.9, including its 6.8.80/6.8.90 pre-releases. The release is read from the installed `kdecoration` at build time, so a rebuild on 6.9 pins to 6.9. A style-only build has no pin.

#### Rebuild policy

| Update | Rebuild needed? | Why |
|---|---|---|
| Plasma bugfix (6.8.x) | No | Binary compatibility is kept within a feature series. |
| KDE Frameworks (monthly KF6) | No | KF6 is binary compatible across releases. |
| Qt minor (6.12 → 6.13) | No | Breeze uses only public Qt APIs. Qt will not load a plugin built against a newer Qt than the one installed, so build against the oldest supported Qt. |
| Plasma feature (6.8 → 6.9) | **Yes** | KDecoration has no binary-compatibility promise, and its soname stays `.so.6` across Plasma 6. A mismatched decoration runs inside KWin, so it can crash the session instead of failing cleanly. |
| Plasma 7 / Qt 7 / KF7 | Yes, plus porting | New APIs. |

**Protea never lets KWin load a mismatched decoration.** The package's version pin makes pacman refuse a Plasma feature upgrade while a decoration build is installed:

```
error: failed to prepare transaction (could not satisfy dependencies)
:: installing kdecoration (6.9.0-1) breaks dependency 'kdecoration<6.8.80' required by protea
```

This also stops `pacman -Syu` from updating anything else, so a new Plasma feature release needs this workflow:

1. `sudo pacman -R protea`. Apps and title bars fall back to Breeze.
2. `sudo pacman -Syu`.
3. Build and install Protea again, against the new Plasma.

Never get around the block with `--ignore` or a partial upgrade. That's exactly the mismatch the pin exists to prevent.

The pin was checked in a simulated pacman database:

| Upgrade attempted | Result |
|---|---|
| `kdecoration` 6.9.0 | Blocked |
| `kdecoration` 6.8.90 | Blocked |
| `kwin` 6.9.0 | Blocked |
| `kdecoration` 6.8.5 | Allowed |
| `kdecoration` 6.9.0 after removing protea | Allowed |

Breeze also changes in each feature release, so each one means rebasing onto the new upstream tag (see [Upgrading upstream](#upgrading-upstream)).

## Installer

A single Go binary that embeds the core theme, the C++ sources and the PKGBUILD, so it works on its own, without the repo.

```sh
go build -o dist/protea-installer .   # dist/ is ignored by git
./dist/protea-installer             # the interface
./dist/protea-installer --dry-run   # same, but only shows what it would change
./dist/protea-installer --uninstall # remove everything without the interface (also takes --dry-run)
go test ./...
```

- **Libraries:**
  - Bubble Tea v2.1.0 (`charm.land/bubbletea/v2`)
  - Bubbles v2.2.1 (spinner)
  - Lip Gloss v2.0.6
- **Toolchain:** Go 1.27.
- **Module:** `proteadesktop.org/protea-theme`, at the repo root. It doesn't need to resolve anywhere while users build from source.
  - The program's `main` package is at the root.
  - `go:embed` can only include files at or below its own package, so [main.go](main.go) at the root embeds `theme/`, `cpp/` and the PKGBUILD. The embedding is in `main.go` itself, so `go run main.go` works as well as `go run .`.
  - `cpp/` is embedded folder by folder, so local `cpp/build*` folders never end up in the binary.
  - A plain `go build` names the binary `protea-theme`, after the module; git ignores it.
- **Code:**
  - [main.go](main.go): the embedded files, flags, and `--uninstall`.
  - [internal/system](internal/system): reads the setup and makes every change.
  - [internal/tui](internal/tui): the Bubble Tea screens.
  - Tests in `internal/` read the same files from the repo; [main_test.go](main_test.go) checks what's embedded.
- **Future scope:** it may later grow into a shared installer for all Protea Desktop themes.

### Look

The installer uses the **Base16** palette: the terminal's own 16 ANSI colors, mapped the way Charm's Huh Base16 theme maps them. It follows whatever theme the terminal uses instead of fixing its own colors. Colors and the logo live in [internal/tui/style.go](internal/tui/style.go).

| Role | ANSI color |
|---|---|
| Body text | white (7) |
| Hints, secondary text, end of the logo | bright black (8) |
| Page titles, start of the logo | cyan (6) |
| Cursor, current step, warnings | yellow (3) |
| Selected option, done | green (2) |
| Page title marker | magenta (5) |
| Failures | bright red (9) |

- **Exception, in true color:** the accent swatches and preview on the Colors page, since they show the exact theme colors being installed.
- **Header:** "PROTEA THEME" in a 5-pixel-tall font drawn with half blocks, like an LED matrix, colored by letter:
  - "PRO" in the terminal's cyan
  - "TEA" in faint cyan
  - "THEME" in bright black, the faded hint color

  The faint text attribute gives the in-between shade, since the terminal's palette has nothing between cyan and gray. A test pins the logo's shape and the color of every column.
- **Progress line** under the header.
- **Page heading:** a colored title, with the page's description underneath.

### Flow

The installer **installs** Protea; it never changes your look. After installing, it reminds you to apply Protea yourself in System Settings. It goes page by page, and nothing changes until the last page confirms.

- **Navigation:** a progress line at the top shows where you are. Enter goes forward, Esc goes back, and choices are kept when going back.
- **The plan:** the pages fill in a plan. The Review page turns it into the exact steps that will run, so what you confirm is what happens.

```
Install:    Welcome → Colors → Options → Review → Install
Uninstall:  Welcome → Remove → Review → Uninstall
Upgrade:    Welcome → Review → Prepare
```

1. **Welcome.** Your system: Plasma version (with a warning if it isn't 6.8), and whether the theme and the C++ components are installed and in use. Then the choices:
   - **Install Protea Dark**, or **Update Protea Dark** when something is installed.
   - **Prepare for Plasma X.Y**: only when `pacman -Qu` shows a Plasma feature upgrade that the version pin blocks.
   - **Uninstall**: always the last choice. It's grayed out ("nothing installed") and skipped by the cursor when there's nothing to remove.
2. **Colors.** The accent color for Protea Dark, from a list of presets, each shown with a swatch and its hex value:
   - Teal `#5ba9ae` (the default: listed first and pre-selected, without a label), then Sage `#8fb573`, Camel `#c4a07a`, Rose `#d0879a` and Lavender `#9b8cd9`
   - A preview line shows a selected item and a link in the chosen accent.
   - For an existing install, it starts on the installed accent.
   - If Protea Dark is in use and the accent changes, it notes that the theme must be applied again to show it. Plasma copies a color scheme's colors when it's applied.
3. **Options.** The theme files (Global Theme, Plasma Style, color scheme) always go into `~/.local/share/` (or `$XDG_DATA_HOME`). Optionally:
   - **Task manager: underline** (the default) or **Task manager: background**: how the task manager marks open apps (see Design → Task manager). It only picks which `widgets/tasks.svg` is installed, so it needs no C++ and no rebuild. For an existing install it starts on the installed style, read from the mark in the installed file. The Review page names the style in the install step.
   - **C++ components**: a single option, unticked by default, that installs both the application style and the window decoration. For an existing install it starts as installed or not.
   - **Rectangular** (the default) or **Round window buttons**: the window decoration's button shape, preselected with the current setting and grayed out without the C++ components. Installing writes it to `~/.config/protearc` and sends KWin the `reloadConfig` signal System Settings sends. Changing only the shape doesn't rebuild anything.
   - **Build again**: only shown when the C++ components are installed, for rebuilding after changing the C++.
   - Notes explain the pin, and say that missing build tools will be installed first.
4. **Review.** A numbered list of the steps, with sudo steps marked, then **Install** (or **Uninstall** / **Prepare**) and **Back**.
   - **Only what changes:** C++ components already built for your Plasma aren't rebuilt.
   - **A mismatched build** (built for another Plasma) is always rebuilt.
   - **Unticking the C++ components** removes them, switching the application style and window decoration back to Breeze first if Protea's are in use.
   - **Builds** run `makepkg --cleanbuild --install` on a temporary copy of the embedded sources.
5. **Install / Uninstall / Prepare.** A spinner per step.
   - Steps that need sudo or show their own output (makepkg, pacman) hand the terminal over with `tea.ExecProcess`, then wait for Enter so the output can be read.
   - Cleanup steps, like removing the temporary build folder, run even after a failure. Everything else after a failed step is skipped.
6. **Done.** Each step's result, errors, and in dry-run mode what would have run. After installing, it shows where to apply Protea in System Settings → Colors & Themes:
   - **Global Theme** → Protea Dark
   - or, to keep icons and title bars, only **Colors** and **Plasma Style** → Protea Dark
   - **Window Decorations** and **Application Style** → Protea, when the C++ components are installed. A reminder in the warning color says to do this after applying the Global Theme, and again whenever it's reapplied, because the Global Theme sets Breeze's.

   After a build, a reminder in the warning color says to log out and back in: KWin keeps the decoration it started with until then. After changing the task manager style while Protea's Plasma Style is in use, one says the task manager may keep its old look until then.

   Enter starts over.

**Remove page (uninstall).** Checkboxes for the theme and for the C++ components, ticked for whatever is installed.

**Prepare (Plasma upgrade).** Switches the application style and window decoration back to Breeze if Protea's are in use, then removes the package so `pacman -Syu` can upgrade Plasma. Then run the installer again to build for the new Plasma.

### Accent color

The installer writes the chosen accent into the installed color files, following the same rules System Settings uses for accent colors (plasma-workspace `kcms/colors/colorsapplicator`), without tinting or accent-colored title bars. The code is in [internal/system/accent.go](internal/system/accent.go).

- **Focus, hover and "active" text:** become the accent in every color group. On selections, active text becomes the selection's readable text color instead.
- **Links:** become the accent, lightened or darkened until their contrast with the group's background reaches 5:1. Links on selections keep their own color.
- **Selections:** the background (and its alternate) is the accent at 70% over the view background. The text on it is white or black, whichever reaches 4.5:1. The inactive (secondary) text on it isn't touched: it stays Protea Dark's own grey, `#a1a1a2`.
- **Buttons' and Complementary's alternate background:** the accent at 70% over their own background. Breeze uses a darker accent there.

**The Plasma Style's own `colors` file only has its existing keys rewritten.** Plasma reads that file on top of the system colors, so keys it leaves out, like the focus colors and selection background, keep following the system color scheme. That includes an accent picked in System Settings.

**Defaults:** the repo's `ProteaDark.colors` and Plasma Style `colors` carry the default Teal accent, written with the same code. The result doesn't depend on the accent a file had before, so switching accents leaves no trace.

### Uninstalling: switching away from Protea first

Removing files that Plasma is still using would leave it pointing at a missing theme. So uninstalling switches away first, using the same tools as System Settings and KWin (checked against plasma-workspace and KWin v6.8.1). Settings you've already changed to something else are left alone.

| If this is selected | Uninstall runs |
|---|---|
| Protea Dark as the **Global Theme** | `plasma-apply-lookandfeel --apply org.kde.breezedark.desktop` |
| Only Protea Dark **colors** / **Plasma Style** | `plasma-apply-colorscheme BreezeDark` / `plasma-apply-desktoptheme default` |
| Protea **application style** | `kwriteconfig6` writes `widgetStyle=Breeze`, then the D-Bus signal System Settings sends: `org.kde.KGlobalSettings.notifyChange` with type 2 (style changed) |
| Protea **window decoration** | `/usr/lib/kwin-applywindowdecoration Breeze`, KWin's own tool, which saves the setting and reloads KWin |

**Why a whole Global Theme switch?** Applying a Global Theme makes Plasma (`KLookAndFeelManager`) write the theme's values into `~/.config/kdedefaults/`, and drop your own values for those settings so the defaults show through. Plasma adds `kdedefaults` to `XDG_CONFIG_DIRS`, and `kdedefaults/package` records which theme it was. Switching only the colors back would leave `kdedefaults` naming Protea Dark. Applying Breeze Dark's Global Theme, as System Settings would, replaces all of it. That also resets the other parts a Global Theme covers, like icons, app style and title bars, to Breeze Dark's.

The installer doesn't remember what you had before Protea, because it never changes your settings. You make those changes in System Settings yourself.

## Design

Step 10 changes Breeze Dark's look one area at a time. Anything not listed here is still Breeze Dark.

### Colors

Protea Dark swaps Breeze's blue-tinted greys for neutral ones. Both color files, `ProteaDark.colors` and the Plasma Style's `colors`, carry the same values.

| Area | Keys | Color |
|---|---|---|
| Window backgrounds, sidebars, panels and popups | `[Colors:Window] BackgroundNormal` | `#2e2e2f` |
| Title bars and header bars | `[Colors:Header] BackgroundNormal`, `[WM] activeBackground` | `#2e2e2f` |
| Title bars and header bars of unfocused windows | `[Colors:Header][Inactive] BackgroundNormal`, `[WM] inactiveBackground` | `#1a1a1b` |
| Views: lists, text fields, editors | `[Colors:View] BackgroundNormal` | `#1a1a1b` |
| Striped list rows | `[Colors:View] BackgroundAlternate` | `#282829` |
| Tooltips | `[Colors:Tooltip] BackgroundNormal` | `#1a1a1b` |

- **Sidebars** have no key of their own. Qt apps draw them with the window color.
- **Plasma's tooltips:** the panel's tooltips use the window colors (`DefaultToolTip.qml` in libplasma 6.8 sets the Window color set). So the Plasma Style's tooltip SVGs, `widgets/tooltip.svg` and its `translucent/`, `opaque/` and `solid/` versions, fill their background with `ColorScheme-TooltipBackground` instead of `ColorScheme-Background`.
- **Accent:** over the new view color, the Teal selection is `#487e82` with black text, because white on it falls just short of 4.5:1. Secondary text on a selection is `#a1a1a2` for every accent (1.6 to 1.9:1 on the selections; Breeze's was 1.05:1).
- **Links on window and header backgrounds** are `#73b5ba`, a lighter Teal: plain Teal on `#2e2e2f` falls just short of the 5:1 links need. Links everywhere else stay `#5ba9ae`.
- **Not changed yet:** buttons (`#292c30`), Complementary (`#202326`), the other alternate backgrounds, and secondary text (`#a1a9b1`) are still Breeze's blue-tinted values.

### Window outline

The thin line around each window is `#3a3a3b`, on focused and unfocused windows alike. This needs the Protea decoration: it's a fixed color, `Outline_Color` in [kdecoration/protea.h](cpp/kdecoration/protea.h), so change it by hand along with the title bar colors. Breeze mixes the outline from the title bar color toward the text color by the color scheme's frame contrast (20%), the same value that draws every frame in apps, so the color scheme alone can't change it. With the stock Breeze decoration the outline is `#575758`, or `#474748` on unfocused windows.

### Corners

Windows, everything Plasma draws on top of them, and the frames inside apps round their corners at **8 px**. Breeze uses 5 px for windows and inside apps, and 6 px for Plasma.

| What | Where it's set |
|---|---|
| Windows: title bar, bottom corners, outline; the shadow at 8.5 px | `Frame_FrameRadius = 4` in [kdecoration/protea.h](cpp/kdecoration/protea.h), in units of KWin's small spacing (always 2 px on Wayland) |
| Panels, popups, tooltips, and the header and footer strips inside popups, with and without compositing | The Plasma Style's frame SVGs, redrawn by [scripts/frame-radius.py](scripts/frame-radius.py) |
| Inside apps: buttons, text fields, combo and spin boxes, menus and their shadows, tooltips, tabs, frames around lists, list highlights, focus rings, drag selection | `Frame_FrameRadius = 8` in [kstyle/proteametrics.h](cpp/kstyle/proteametrics.h), in pixels |

- **Changing the radius:** set the decoration's `Frame_FrameRadius` to half the new radius and the app style's to the radius itself, and run `scripts/frame-radius.py <radius>`. `TestCornerRadiusMatches` fails if any of them differ. The decoration and app style need rebuilding.
- **What the script redraws:** each frame's corners, as true circular arcs like the decoration's, with edges as thick as the radius. It also redraws the blur mask, 1 px smaller as in Breeze, and the shadow, using Breeze's own falloff. Each file keeps its colors, opacity and margins, so the content inside doesn't move. The script needs Inkscape to measure the files.
- **Without compositing** (the `opaque/` versions, only used on X11 with compositing off): there's no transparency to smooth a curve with, so the script draws their corners in whole pixels, as a staircase with the outline along it, and the window's shape (the mask) uses the same pixels. Their radius has to be a whole number of pixels.
- **Bottom border:** the decoration's radius also sets the bottom border with the "No side borders" setting, as in upstream's formula: 2 px plus the radius in units, so 6 px now (Breeze: 4.5 px).

**Not changed yet** (to do later):

| What | Radius now | Where it's set |
|---|---|---|
| Plasma's own controls in panel popups: buttons and list highlights (text fields are done, see Widget popups) | 5 px, with 7 px focus rings | The Plasma Style's `widgets/button.svg` and `widgets/viewitem.svg` |
| Checkboxes inside apps | 4 px | `CheckBox_Radius` in [kstyle/proteametrics.h](cpp/kstyle/proteametrics.h), kept at Breeze's value when `Frame_FrameRadius` went to 8 |
| Desktop widget backgrounds | 4 px | The Plasma Style's `widgets/background.svg` |
| Kirigami's cards in System Settings and Discover | 5 px | Kirigami itself, not the theme |

### Title bar buttons

Minimize, maximize, restore and close use Windows-style symbols, in [kdecoration/proteabutton.cpp](cpp/kdecoration/proteabutton.cpp) (`Button::drawIcon`). They sit in a 10 × 10 px box like Windows 11's, drawn with 1 px lines on pixel centers so they stay sharp at 100% scale.

| Button | Symbol |
|---|---|
| Minimize | a 10 px line |
| Maximize | a 10 × 10 px square with sharp corners |
| Restore (maximized window) | two 8 × 8 px squares with sharp corners, offset by 2 px, the back one hidden behind the front |
| Close | a 10 × 10 px X |

- **See-through:** the symbols are drawn at 80% opacity (`Button_SymbolOpacity`), and Close's red too (`Button_CloseOpacity`).
- **Hover:** white at 30% opacity behind the button, in Breeze's round shape, fading in and out like Breeze's. The symbol keeps its color. Close turns the color scheme's red (`ForegroundNegative`, `#da4453`) with the white X kept.
- **Pressed:** white at 20%, or a darker red for Close. The opacities are `Button_HoverOpacity` and `Button_PressedOpacity` in [kdecoration/protea.h](cpp/kdecoration/protea.h).
- **Size and spacing:** buttons are 24 px with the default font, 8 px apart (Breeze: 20 px, 4 px apart), from `Button_DefaultSize` (2.4 grid units, the title font's "M" height) and `TitleBar_ButtonSpacing` in [kdecoration/protea.h](cpp/kdecoration/protea.h). The symbols keep Breeze's 10 px size, centered; only the button and its hover circle grow. The margins above, below and beside the buttons are 2 px (Breeze: 4 px), so the title bar stays 28 px and the outermost buttons' symbols, like Close's X, sit exactly where Breeze has them.
- **Shapes:** the decoration's own "Button shape" setting (`ButtonShape` in `~/.config/protearc`, in the decoration's settings window and in the installer). **Rectangular** (the default) is Windows-like: 32 × 28 px buttons (`Button_RectangleWidth`, 3.2 grid units, kept even in pixels so the symbols stay sharp, and the full title bar height), side by side with no gaps, flush with the title bar's top and outer edges, so Close sits in the corner. The highlight is fainter on the bigger rectangles (white at 10% hover, 7% pressed), Close still turns red, and its red follows the window's rounded corner. The buttons on the left get the same treatment. **Round** is the above.
- **Previewing:** KWin's decoration preview module (`org.kde.kwin.private.kdecoration`, what System Settings uses) can draw a local build offscreen, with `QT_PLUGIN_PATH` pointing at it and a temporary `XDG_CONFIG_HOME`. That's how the shapes were checked before installing.
- **Unchanged from Breeze:** the other buttons' symbols (keep above, on all desktops and so on), and the decoration's "outline close button" option.

### See-through title bar

The title bar, and the window borders when there are any, are drawn at **80% opacity**, and KWin's Blur effect blurs what's behind them. The setting is "Title bar opacity" in the decoration's settings window (`TitleBarOpacity` in `~/.config/protearc`, 50 to 100%, where 100% is solid).

- **How:** the decoration fills the title bar in one pass with the see-through color (Breeze's overlapping layers would add up into darker bands), and gives KWin a blur region in the window's shape with its rounded corners (`Decoration::updateBlurRegion`). KWin only blurs the decoration, never the window's contents, so nothing changes inside apps.
- **The shadow stops at the window's edge.** Breeze tucks the window shadow 2 px under the window to hide hairline gaps at fractional display scales, and the shadow is darkest there. Under a see-through title bar that showed as a dark line along the edges, so with an opacity below 100% the shadow is cut out exactly at the edge instead (`Decoration::createShadowObject`). At fractional scales a hairline gap between shadow and window may show instead.
- **Solid when maximized:** there's nothing worth seeing behind a maximized window, and its title bar then meets the app's toolbar seamlessly. It's also solid without compositing.
- **Apps' toolbars stay solid.** KDE apps paint their toolbar right under the title bar in the same color. That belongs to the app, not the decoration, so on a normal window a frosted strip sits above a solid toolbar. Making toolbars see-through as well would need the app style to make every app's window transparent (what Kvantum does), which breaks some apps.
- **Without the Blur effect**, what's behind shows through unblurred. The blur strength is a KWin setting: System Settings → Desktop Effects → Blur.

### Widget popups

Inside Plasma's popups and widgets, there are no separator lines, headers and footers blend into the popup, text fields are borderless and light, hover highlights have no outline, and the selected tab is marked by a grey background instead of an accent line. [scripts/widget-frames.py](scripts/widget-frames.py) draws all of it from the values at its top; change them there and rerun it (it needs Inkscape).

| What | Look | File |
|---|---|---|
| Text fields, search bars included | 8 px corners, text color at 10%, no outline. Hover: 1 px outline in the hover color at 30%. Focus: 1 px outline in the focus color with a 2 px halo at 33% outside it (10 px corners) | `widgets/lineedit.svg` |
| Selected tab, on all four sides | Text color at 10% with 8 px corners; no accent line (Breeze: a 3 px line along its outer edge) | `widgets/tabbar.svg` |
| Lines between sections | Transparent, keeping their size | `widgets/line.svg` |
| Popup headers and footers | Transparent, background and line, keeping their size; `scripts/frame-radius.py` keeps them that way (Breeze: the window color at 75% with a line) | `widgets/plasmoidheading.svg` |
| Buttons and flat buttons under the pointer | The accent at 30%, no outline (Breeze: a 1 px selection-color outline). A raised button's hover is drawn over its resting frame, so it first covers the resting outline with an opaque ring in the button color | `widgets/button.svg` |
| List items under the pointer, like the launcher's apps | The accent at 30%, no outline (Breeze: plus a 1 px accent outline) | `widgets/viewitem.svg` |

- **Every Plasma text field** uses `lineedit.svg`: search bars, password and rename fields, editable combo boxes, spin boxes and text areas. Fields keep their size: their padding comes from the frame's hint margins, not its corners.
- **Section lines** from `line.svg` are the ones in list section headings, expandable list items, and the Kickoff, Kicker and Konsole-profiles widgets. Lines drawn with Kirigami's separator (the Weather widget has two) come from the color scheme, appear inside apps too, and stay.
- **Still with an outline:** pressed buttons (`pressed`, `toolbutton-pressed`: the selection color at 33% with a 1 px outline). Default list rows (`widgets/listitem.svg`) already hover with a plain 20% accent tint.
- **Protea Launcher** draws its own pill-shaped search bar while its "Round search bar corners" option is on (the default), so it only gets this look with that option off.
- **Placeholder text** ("Search…") is about 4.2:1 against the field at 10%, and typed text about 9:1.

### Task manager

There are two styles, chosen in the installer. Breeze tints every open app and draws a 3 px line along the top.

**Underline** (the default): in a bottom panel, each open app is marked by a **2 px line along the bottom edge**. Only the focused app, and an app under the pointer, get a tint behind them.

| App | At rest | Under the pointer |
|---|---|---|
| Running | grey line (text color, 35%) | grey tint (text color, 10%) behind the line |
| Minimized | the same as running | the same as running |
| Focused | accent tint 45%, accent line 90% | accent tint 34%, accent line 100% |
| Needs attention | orange line (`ForegroundNeutral`, 80%) | grey tint behind the orange line |
| Pinned, not running | nothing | nothing; the icon brightens |

**Background:** each open app gets a **see-through tint** behind it, with square corners. The 2 px line along the bottom only appears under the pointer.

| App | At rest | Under the pointer |
|---|---|---|
| Running | grey tint (text color, 8%) | the tint, plus a grey line (40%) |
| Minimized | the same as running | the same as running |
| Focused | accent tint 45% | the tint, plus an accent line (100%) |
| Needs attention | orange tint 35% | the tint, plus an orange line (80%) |
| Pinned, not running | nothing | nothing; the icon brightens |

- **Where:** the underline style is the Plasma Style's own `widgets/tasks.svg`; the background style is `theme/variants/tasks-background.svg`, which the installer copies over it when chosen. [scripts/task-frames.py](scripts/task-frames.py) draws both from its `STYLES` table, starting the background file as a copy of the underline one. Change a color or opacity there and rerun it; it needs Inkscape to measure the files.
- **Telling them apart:** the script marks each file with its style (`<desc id="protea-task-style">`). The installer reads the mark from the installed file to start on the installed style.
- **Each state has its own hover frame** (`focus-hover`, `normal-hover`, `minimized-hover`, `attention-hover`), so an app keeps its line under the pointer. The task manager looks for `<state>-hover` before the shared `hover`, which is now only a fallback.
- **Top and side panels keep Breeze's look**, from their `north-`, `east-` and `west-` frames. The task manager tries the plain `<state>-hover` before `<edge>-hover`, so the script gives each edge a copy of its own hover frame under each new name. `TestTaskHoverFramesPerEdge` fails if one is missing.
- **2 px between apps.** The task manager puts apps right next to each other, and each frame leaves 1 px clear on each side. A 1 px gap would need the clear column on one side only, which leaves the frame half a pixel off the icon, since Plasma always centers the icon in its slot.
- **Sizes:** every frame keeps Breeze's piece sizes, so icons don't move and the `normal-hint-*-margin` margins still apply.

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
│   ├── rebrand-cpp.py          renames an imported Breeze C++ tree to Protea
│   ├── frame-radius.py         redraws the Plasma Style's panel, popup and tooltip corners at a radius
│   ├── task-frames.py          redraws the task manager's frames for both styles
│   └── widget-frames.py        redraws widget popups' text fields, selected tabs and hover highlights, and hides their separator lines, headers and footers
├── theme/                      core theme
│   ├── color-schemes/
│   ├── desktoptheme/
│   ├── look-and-feel/
│   ├── variants/               files the installer swaps into the Plasma Style by choice (the background task manager)
│   └── LICENSES/               license texts from libplasma + plasma-workspace
├── cpp/                        forked breeze repo (kstyle, kdecoration, libproteacommon)
├── packaging/arch/             PKGBUILD for the C++ part
├── go.mod, go.sum              Go module proteadesktop.org/protea-theme
├── main.go                     the installer program (Go + Bubble Tea); embeds theme/, cpp/ and the PKGBUILD
├── main_test.go                checks what's embedded, and that window and Plasma corners match
└── internal/
    ├── system/                 reads the setup and makes the changes
    └── tui/                    the installer's screens
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
| 8 | Add the `WITH_STYLE` CMake option and write the PKGBUILD. Install the package, verify Breeze and Protea both appear under Application Style and Window Decorations, and test reverting to Breeze. | Option, PKGBUILD and version pin done; all component combinations build, the package has no file conflicts, and the pin was checked in a simulated pacman database. Installing and testing it is up to the repo owner. |
| 9 | Build the Go installer | Done. Tests cover the switching logic against temporary configs, every page, dry-run mode, and building the package from the embedded sources. Running it for real on the desktop is up to the repo owner. |
| 10 | Design work: colors, SVGs, style metrics (`kstyle/proteametrics.h`), decoration drawing (corner radius in `kdecoration/protea.h`) | In progress. Neutral greys, tooltips, the window outline, 8 px corners on windows, panels, popups, tooltips and inside apps, Windows-style title bar buttons, a see-through, blurred title bar, the task manager's bottom lines, and quieter widget popups (no separator lines, light text fields, grey selected tabs) are done (see Design). |

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
- Plasma Style, and its variants in `theme/variants/`: LGPL.
- Global Theme: GPL-2.0-or-later.
- Color scheme: per its SPDX headers.

## Open items

- Register `proteadesktop.org` before the first public release. The `org.proteadesktop` IDs imply ownership of that domain.
- Light variant later. IDs are already shaped for it (`protea-light`, `ProteaLight`).
- Replace the Global Theme previews (`contents/previews/`). They are still Breeze Dark screenshots.
- Colors live in two files: the color scheme (`ProteaDark.colors`) and the Plasma Style's own `colors`. The installer keeps the accent in sync in both, but other color changes need making in both by hand, or the panels won't match the apps.
- The Plasma Style may need its SVG cache cleared after updating the theme files, so changes show without logging out. Check this during the design work.
- **Global Theme and the C++ components (deferred).** The Global Theme names Breeze's window decoration and app style, so people without the C++ components never get a setting pointing at a missing plugin. When the components are installed, users pick Protea's by hand (the Done page reminds them). A possible fix: the installer rewrites the *installed* Global Theme's defaults to name Protea's decoration and style when it builds the components, and back to Breeze when it removes them, like it does for the accent. That follows KDE's own Oxygen Global Theme, which ships in the same package as the decoration and style it names. Checked in Plasma 6.8.1: if a Global Theme names a missing app style, System Settings skips it. If it names a missing decoration, System Settings writes it anyway, and KWin keeps the current decoration, falling back to Breeze at the next login.

## Project conventions

- **Repo management.** The repo owner manages git and GitHub. Tooling doesn't create repos, commit or push. Work stops at checkpoints so the owner can commit.
- **IDs are permanent once released.** Users' configs store them (for example `LookAndFeelPackage=` in `kdeglobals`), so renaming one later breaks existing installs.
