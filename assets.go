// Package protea embeds the theme files, the C++ sources and the Arch
// PKGBUILD, so the installer can be a single binary.
package protea

import "embed"

// Files holds the embedded files at their paths in this repo.
//
// cpp/ is listed folder by folder so local CMake build folders inside it
// (cpp/build*, ignored by git) never end up in the binary.
//
//go:embed all:theme
//go:embed cpp/CMakeLists.txt cpp/build-info.in cpp/README.md cpp/AUTHORS
//go:embed all:cpp/kstyle all:cpp/kdecoration all:cpp/libproteacommon all:cpp/po all:cpp/LICENSES
//go:embed packaging/arch/PKGBUILD
var Files embed.FS
