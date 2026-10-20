// Command protea-installer installs Protea Dark and, optionally, builds its
// C++ application style and window decoration.
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"proteadesktop.org/protea-theme/internal/system"
	"proteadesktop.org/protea-theme/internal/tui"
)

// files holds the theme files, the C++ sources and the Arch PKGBUILD at
// their paths in this repo, so the installer works without the repo.
//
// cpp/ is listed folder by folder so local CMake build folders inside it
// (cpp/build*, ignored by git) never end up in the binary.
//
//go:embed all:theme
//go:embed cpp/CMakeLists.txt cpp/build-info.in cpp/README.md cpp/AUTHORS
//go:embed all:cpp/kstyle all:cpp/kdecoration all:cpp/libproteacommon all:cpp/po all:cpp/LICENSES
//go:embed packaging/arch/PKGBUILD
var files embed.FS

func main() {
	uninstall := flag.Bool("uninstall", false, "switch to Breeze Dark where Protea is in use, then remove Protea Dark and the protea package, without the interface")
	dryRun := flag.Bool("dry-run", false, "show what would change without changing anything")
	flag.Parse()

	runner := &system.Runner{DryRun: *dryRun}
	if *uninstall {
		if !uninstallAll(runner) {
			os.Exit(1)
		}
		return
	}
	if _, err := tea.NewProgram(tui.New(files, runner, system.ReadStatus)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// uninstallAll removes everything Protea installed, printing each step.
// It reports whether every step succeeded.
func uninstallAll(r *system.Runner) bool {
	st := system.ReadStatus()
	ok := true
	report := func(name string, err error) {
		if err != nil {
			fmt.Printf("✗ %s: %v\n", name, err)
			ok = false
		} else {
			fmt.Printf("✓ %s\n", name)
		}
		for _, l := range r.TakeLog() {
			fmt.Println("  " + l)
		}
	}

	if st.StyleActive || st.DecorationActive {
		report("Switch the application style and window decoration back to Breeze", r.RevertComponents())
	}
	if st.Package != "" {
		cmd := r.RemoveCommand(false)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		report("Remove the protea package", cmd.Run())
	}
	report("Switch away from Protea Dark to Breeze Dark", r.RevertCore())
	if st.CoreInstalled {
		report("Remove the theme files", r.UninstallCore())
	}
	return ok
}
