package main

import (
	"io/fs"
	"strings"
	"testing"

	"proteadesktop.org/protea-theme/internal/system"
)

func TestEmbeddedFiles(t *testing.T) {
	for _, p := range []string{
		"theme/look-and-feel/" + system.GlobalThemeID + "/metadata.json",
		"theme/desktoptheme/" + system.PlasmaStyleID + "/metadata.json",
		"theme/color-schemes/" + system.ColorSchemeID + ".colors",
		"cpp/CMakeLists.txt",
		"cpp/build-info.in",
		"cpp/kstyle/proteastyle.cpp",
		"cpp/po",
		"packaging/arch/PKGBUILD",
	} {
		if _, err := fs.Stat(files, p); err != nil {
			t.Errorf("not embedded: %v", err)
		}
	}
	entries, _ := fs.ReadDir(files, "cpp")
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "build") {
			t.Errorf("a local build folder was embedded: cpp/%s", e.Name())
		}
	}
}
