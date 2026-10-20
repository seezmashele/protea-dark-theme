// Package system reads and changes the user's Plasma setup: it installs the
// core theme files, switches appearance settings, and builds and removes the
// C++ package.
package system

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// IDs of what Protea installs. They are permanent once released.
const (
	GlobalThemeID = "org.proteadesktop.protea-dark"
	PlasmaStyleID = "protea-dark"
	ColorSchemeID = "ProteaDark"
	WidgetStyleID = "Protea"
	// DecorationID is KWin's theme name for the org.proteadesktop.protea
	// decoration plugin (its display name).
	DecorationID = "Protea"
	PackageName  = "protea"

	// TargetPlasma is the Plasma feature release Protea is forked from.
	TargetPlasma = "6.8"
)

// BuildInfoPath is where the C++ package records what it was built against.
var BuildInfoPath = "/usr/share/protea/build-info"

// BuildTools are the packages makepkg needs to build the C++ package.
var BuildTools = []string{"base-devel", "cmake", "extra-cmake-modules"}

// Runner makes changes to the system. With DryRun set, it records what it
// would change instead of changing it; read-only queries still run.
type Runner struct {
	DryRun bool

	// stub, when set, receives each command instead of it being run. Unlike
	// DryRun, the saved-settings file is still written. For tests.
	stub func(cmd []string)

	mu  sync.Mutex
	log []string
}

func (r *Runner) logf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, fmt.Sprintf(format, args...))
}

// TakeLog returns and clears the dry-run messages recorded so far.
func (r *Runner) TakeLog() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	log := r.log
	r.log = nil
	return log
}

// query runs a read-only command and returns its trimmed output.
func query(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// change runs a command that modifies the system.
func (r *Runner) change(name string, args ...string) error {
	if r.stub != nil {
		r.stub(append([]string{name}, args...))
		return nil
	}
	if r.DryRun {
		r.logf("would run: %s", strings.Join(append([]string{name}, args...), " "))
		return nil
	}
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("%s: %w", name, err)
		}
		return fmt.Errorf("%s: %w: %s", name, err, msg)
	}
	return nil
}

// Interactive returns a command for steps that need the terminal, such as
// sudo password prompts or makepkg's build output. With pause set, it waits
// for Enter afterwards so the output can be read before the installer
// redraws.
func (r *Runner) Interactive(pause bool, dir string, env []string, name string, args ...string) *exec.Cmd {
	script := `"$@"`
	if r.DryRun {
		script = `echo "Dry run, would run: $*"`
		if dir != "" {
			script += fmt.Sprintf(`; echo "  in %s"`, dir)
		}
		for _, e := range env {
			script += fmt.Sprintf(`; echo "  with %s"`, e)
		}
	}
	if pause {
		script += `; status=$?; echo; read -rp "Press Enter to return to the installer "; exit $status`
	}
	cmd := exec.Command("bash", append([]string{"-c", script, "bash", name}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func xdgDir(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	return filepath.Join(homeDir(), fallback)
}

// DataHome is where the core theme is installed (~/.local/share).
func DataHome() string { return xdgDir("XDG_DATA_HOME", ".local/share") }

func stateHome() string { return xdgDir("XDG_STATE_HOME", ".local/state") }

// libexec finds a helper Plasma installs outside $PATH.
func libexec(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return "/usr/lib/" + name
}

// Feature returns the feature release of a version: "6.8.0-1" gives "6.8".
func Feature(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

// parseKeyValues reads KEY=VALUE lines.
func parseKeyValues(text string) map[string]string {
	values := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok {
			values[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return values
}

// copyTree copies the embedded folder src to dest, replacing dest.
func copyTree(files fs.FS, src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	sub, err := fs.Sub(files, src)
	if err != nil {
		return err
	}
	return os.CopyFS(dest, sub)
}

// copyFile copies the embedded file src to dest.
func copyFile(files fs.FS, src, dest string) error {
	data, err := fs.ReadFile(files, src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
