package system

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallCore copies the core theme into DataHome, replacing any older
// copy, puts the task manager style (TasksUnderline or TasksBackground) in
// place, and writes the accent color (#rrggbb) into its color files.
func (r *Runner) InstallCore(files fs.FS, accent, tasks string) error {
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
	if err := r.writeTaskStyle(files, tasks); err != nil {
		return err
	}
	return r.writeAccent(accent)
}

// writeTaskStyle puts the style's tasks.svg in place of the Plasma Style's
// own, which is the underline style.
func (r *Runner) writeTaskStyle(files fs.FS, style string) error {
	if style == TasksUnderline {
		return nil
	}
	src, ok := taskStyleFiles[style]
	if !ok {
		return fmt.Errorf("unknown task manager style %q", style)
	}
	dest := filepath.Join(DataHome(), tasksSVG)
	if r.DryRun {
		r.logf("would copy %s to %s", src, dest)
		return nil
	}
	return copyFile(files, src, dest)
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

// RevertCore switches away from Protea Dark before its files are removed.
// If Protea Dark is the Global Theme, it applies the Breeze Dark Global
// Theme, which covers every part. Otherwise it switches the color scheme
// and Plasma Style back to Breeze Dark's where Protea Dark's are selected.
// Call it before UninstallCore.
func (r *Runner) RevertCore() error {
	if switched, err := r.switchAway(&lookAndFeel, GlobalThemeID); switched || err != nil {
		return err
	}
	_, colorsErr := r.switchAway(&colorScheme, ColorSchemeID)
	_, styleErr := r.switchAway(&plasmaStyle, PlasmaStyleID)
	return errors.Join(colorsErr, styleErr)
}

// RevertComponents switches the application style and window decoration
// back to Breeze where Protea's are selected. Call it before removing the
// package.
func (r *Runner) RevertComponents() error {
	_, styleErr := r.switchAway(&widgetStyle, WidgetStyleID)
	_, decorationErr := r.switchAway(&decoration, DecorationID)
	return errors.Join(styleErr, decorationErr)
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

// BuildCommand builds the package in dir, with both the application style
// and the window decoration, and installs it with sudo.
func (r *Runner) BuildCommand(dir string) *exec.Cmd {
	return r.Interactive(true, dir, nil, "makepkg", "--cleanbuild", "--install")
}

// RemoveCommand removes the package with sudo.
func (r *Runner) RemoveCommand(pause bool) *exec.Cmd {
	return r.Interactive(pause, "", nil, "sudo", "pacman", "-R", PackageName)
}

// ToolsCommand installs the build tools with sudo.
func (r *Runner) ToolsCommand() *exec.Cmd {
	return r.Interactive(true, "", nil, "sudo", append([]string{"pacman", "-S", "--needed"}, BuildTools...)...)
}
