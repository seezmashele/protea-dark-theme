# Protea C++ components

The optional compiled parts of Protea Dark, forked from KDE's [Breeze](https://invent.kde.org/plasma/breeze). See [../UPSTREAM.md](../UPSTREAM.md) for the exact upstream version.

- **Application style** (`kstyle/`): Qt style plugin `Protea` (`protea6.so`), plus its settings page and the `protea-settings6` app.
- **Window decoration** (`kdecoration/`): KWin decoration plugin `org.proteadesktop.protea`, plus its settings page.
- **Common code** (`libproteacommon/`): shadow rendering shared by both, compiled into each plugin.

Both take their colors from the active color scheme. They install alongside Breeze without conflicting, and store settings in `protearc`.

## Building

Build dependencies on Arch: `cmake` and `extra-cmake-modules`.

```sh
cmake -S cpp -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_TESTING=OFF
cmake --build build -j"$(nproc)"
```

To build only one component, add `-DWITH_STYLE=OFF` or `-DWITH_DECORATIONS=OFF`. Each build installs `/usr/share/protea/build-info`, which records the Qt and KDecoration versions it was compiled against. On Arch, build a package with [../packaging/arch/PKGBUILD](../packaging/arch/PKGBUILD) instead.

The plugins must be rebuilt after each Plasma feature release (for example 6.8 → 6.9). See [../PLAN.md](../PLAN.md).

## Compatibility with Breeze

KDE apps set widget properties such as `_breeze_force_frame` and `_breeze_borders_sides` to ask the style for frames and separators. Protea keeps reading these Breeze names, so those apps look right.
