// Command protea-installer installs Protea Dark and, optionally, builds its
// C++ application style and window decoration.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	protea "proteadesktop.org/protea-theme"
	"proteadesktop.org/protea-theme/installer/internal/system"
	"proteadesktop.org/protea-theme/installer/internal/tui"
)

func main() {
	uninstall := flag.Bool("uninstall", false, "switch back to your previous settings and remove Protea Dark and the protea package, without the interface")
	dryRun := flag.Bool("dry-run", false, "show what would change without changing anything")
	flag.Parse()

	runner := &system.Runner{DryRun: *dryRun}
	if *uninstall {
		if !uninstallAll(runner) {
			os.Exit(1)
		}
		return
	}
	if _, err := tea.NewProgram(tui.New(protea.Files, runner, system.ReadStatus)).Run(); err != nil {
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
		report("Switch back to your previous application style and window decoration", r.RevertComponents())
	}
	if st.Package != "" {
		cmd := r.RemoveCommand(false)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		report("Remove the protea package", cmd.Run())
	}
	report("Switch back to your previous theme settings", r.RevertCore())
	if st.CoreInstalled {
		report("Remove the theme files", r.UninstallCore())
	}
	return ok
}
