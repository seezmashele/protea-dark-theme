package system

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallCore copies the core theme into DataHome, replacing any older copy.
func (r *Runner) InstallCore(files fs.FS) error {
	for _, c := range coreFiles {
		dest := filepath.Join(DataHome(), c.dest)
		if r.DryRun {
			r.logf("would copy %s to %s", c.src, dest)
			continue
		}
		info, err := fs.Stat(files, c.src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = copyTree(files, c.src, dest)
		} else {
			err = copyFile(files, c.src, dest)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// UninstallCore removes the core theme files. Call RevertCore first.
func (r *Runner) UninstallCore() error {
	for _, c := range coreFiles {
		dest := filepath.Join(DataHome(), c.dest)
		if r.DryRun {
			r.logf("would remove %s", dest)
			continue
		}
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	return nil
}

// ApplyColors switches to Protea's color scheme and Plasma Style, leaving
// everything else alone.
func (r *Runner) ApplyColors() error {
	return errors.Join(
		r.set(&colorScheme, ColorSchemeID),
		r.set(&plasmaStyle, PlasmaStyleID),
	)
}

// ApplyGlobalTheme applies the whole Protea Dark Global Theme, which also
// switches icons, cursors, application style and window decoration to the
// values in its defaults.
func (r *Runner) ApplyGlobalTheme() error {
	saved := loadSaved()
	for s, value := range themeValues {
		remember(saved, s, value)
	}
	for s, value := range globalThemeDefaults {
		remember(saved, s, value)
	}
	if err := r.storeSaved(saved); err != nil {
		return err
	}
	return r.change("plasma-apply-lookandfeel", "--apply", GlobalThemeID)
}

// RevertCore switches every setting Protea Dark's theme files provide back
// to the user's previous choice, and undoes the Global Theme's other
// changes where they are still in place.
func (r *Runner) RevertCore() error {
	var errs []error
	for s, value := range themeValues {
		errs = append(errs, r.restore(s, false, value))
	}
	for s, value := range globalThemeDefaults {
		errs = append(errs, r.restore(s, true, value))
	}
	return errors.Join(errs...)
}

// UseStyle switches the application style to Protea.
func (r *Runner) UseStyle() error { return r.set(&widgetStyle, WidgetStyleID) }

// UseDecoration switches the window decoration to Protea.
func (r *Runner) UseDecoration() error { return r.set(&decoration, DecorationID) }

// RevertStyle switches the application style back to the user's previous
// choice, if Protea's is in use.
func (r *Runner) RevertStyle() error { return r.restore(&widgetStyle, false, WidgetStyleID) }

// RevertDecoration switches the window decoration back to the user's
// previous choice, if Protea's is in use.
func (r *Runner) RevertDecoration() error { return r.restore(&decoration, false, DecorationID) }

// RevertComponents switches the application style and window decoration
// back to the user's previous choice, where Protea's are in use.
func (r *Runner) RevertComponents() error {
	return errors.Join(r.RevertStyle(), r.RevertDecoration())
}

// PrepareBuild copies the C++ sources and PKGBUILD into a new temporary
// folder, laid out as in the repo so the PKGBUILD finds ../../cpp. It
// returns the folder to run makepkg in.
func (r *Runner) PrepareBuild(files fs.FS) (string, error) {
	if r.DryRun {
		r.logf("would copy the C++ sources and PKGBUILD to a temporary folder")
		return os.TempDir(), nil
	}
	root, err := os.MkdirTemp("", "protea-build-")
	if err != nil {
		return "", err
	}
	if err := copyTree(files, "cpp", filepath.Join(root, "cpp")); err != nil {
		return "", err
	}
	dir := filepath.Join(root, "packaging", "arch")
	if err := copyFile(files, "packaging/arch/PKGBUILD", filepath.Join(dir, "PKGBUILD")); err != nil {
		return "", err
	}
	return dir, nil
}

// CleanBuild removes the folder PrepareBuild made.
func (r *Runner) CleanBuild(dir string) error {
	if r.DryRun {
		return nil
	}
	return os.RemoveAll(filepath.Dir(filepath.Dir(dir)))
}

func onOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

// BuildCommand builds the package in dir and installs it with sudo.
func (r *Runner) BuildCommand(dir string, style, decoration bool) *exec.Cmd {
	env := []string{"PROTEA_WITH_STYLE=" + onOff(style), "PROTEA_WITH_DECORATIONS=" + onOff(decoration)}
	return r.Interactive(true, dir, env, "makepkg", "--cleanbuild", "--install")
}

// RemoveCommand removes the package with sudo.
func (r *Runner) RemoveCommand(pause bool) *exec.Cmd {
	return r.Interactive(pause, "", nil, "sudo", "pacman", "-R", PackageName)
}

// ToolsCommand installs the build tools with sudo.
func (r *Runner) ToolsCommand() *exec.Cmd {
	return r.Interactive(true, "", nil, "sudo", append([]string{"pacman", "-S", "--needed"}, BuildTools...)...)
}
