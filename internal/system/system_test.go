package system

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// repoFiles are this repo's files, which the installer embeds.
var repoFiles = os.DirFS("../..")

func TestFeature(t *testing.T) {
	for in, want := range map[string]string{"6.8.0-1": "6.8", "6.8.90-1": "6.8", "6.10.2": "6.10", "6": "6"} {
		if got := Feature(in); got != want {
			t.Errorf("Feature(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStatusChecks(t *testing.T) {
	built := map[string]string{"WITH_STYLE": "ON", "WITH_DECORATIONS": "ON", "KDECORATION_VERSION": "6.8.0"}
	s := Status{Package: "0.1.0-1", Build: built, Kdecoration: "6.8.0-1", KdecorationUpgrade: "6.8.5-1"}
	if s.Mismatch() || s.UpgradeBlocked() {
		t.Errorf("a 6.8 bugfix upgrade should be neither a mismatch nor blocked")
	}
	s.KdecorationUpgrade = "6.9.0-1"
	if !s.UpgradeBlocked() {
		t.Errorf("a 6.9 upgrade should be blocked")
	}
	s.Kdecoration = "6.9.0-1"
	if !s.Mismatch() {
		t.Errorf("a 6.8 build on 6.9 should be a mismatch")
	}
	s.Build = map[string]string{"WITH_STYLE": "ON", "WITH_DECORATIONS": "OFF"}
	if s.Mismatch() || s.UpgradeBlocked() {
		t.Errorf("a style-only build is never pinned")
	}
}

func TestInstallAndUninstallCore(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	r := &Runner{}
	if coreInstalled() {
		t.Fatal("installed before installing")
	}
	if err := r.InstallCore(repoFiles); err != nil {
		t.Fatal(err)
	}
	if !coreInstalled() {
		t.Fatal("not installed after installing")
	}
	metadata := filepath.Join(DataHome(), "plasma/look-and-feel", GlobalThemeID, "metadata.json")
	if _, err := os.Stat(metadata); err != nil {
		t.Errorf("Global Theme not copied: %v", err)
	}
	// Installing again replaces the files.
	if err := r.InstallCore(repoFiles); err != nil {
		t.Fatal(err)
	}
	if err := r.UninstallCore(); err != nil {
		t.Fatal(err)
	}
	if coreInstalled() {
		t.Error("still installed after uninstalling")
	}
}

func TestPrepareBuild(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	r := &Runner{}
	dir, err := r.PrepareBuild(repoFiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"PKGBUILD", "../../cpp/CMakeLists.txt", "../../cpp/po"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("build folder incomplete: %v", err)
		}
	}
	if err := r.CleanBuild(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Dir(dir))); !os.IsNotExist(err) {
		t.Errorf("build folder not removed")
	}
}

// fakeSetup points KDE's config reader at temporary folders, writes the
// given config files, and returns a runner that records commands instead
// of running them. XDG_CONFIG_DIRS is replaced too: Plasma adds
// ~/.config/kdedefaults to it, where an applied Global Theme's values live.
func fakeSetup(t *testing.T, configs map[string]string) (*Runner, *[][]string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_DIRS", t.TempDir())
	writeConfigs(t, configs)
	var ran [][]string
	return &Runner{stub: func(cmd []string) { ran = append(ran, cmd) }}, &ran
}

func writeConfigs(t *testing.T, configs map[string]string) {
	t.Helper()
	for name, text := range configs {
		if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// commands returns the recorded commands as sorted strings, with helper
// paths reduced to their names.
func commands(ran [][]string) []string {
	var out []string
	for _, c := range ran {
		out = append(out, filepath.Base(c[0])+" "+strings.Join(c[1:], " "))
	}
	slices.Sort(out)
	return out
}

func check(t *testing.T, ran *[][]string, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := commands(*ran); !reflect.DeepEqual(got, want) {
		t.Errorf("commands:\n got %q\nwant %q", got, want)
	}
	*ran = nil
}

func TestRevertCoreReplacesGlobalTheme(t *testing.T) {
	r, ran := fakeSetup(t, map[string]string{
		"kdeglobals": "[General]\nColorScheme=ProteaDark\n[KDE]\nLookAndFeelPackage=" + GlobalThemeID + "\n",
		"plasmarc":   "[Theme]\nname=protea-dark\n",
	})
	if err := r.RevertCore(); err != nil {
		t.Fatal(err)
	}
	// The Breeze Dark Global Theme covers colors and Plasma Style too.
	check(t, ran, "plasma-apply-lookandfeel --apply org.kde.breezedark.desktop")
}

func TestRevertCoreSwitchesColorsAndStyle(t *testing.T) {
	r, ran := fakeSetup(t, map[string]string{
		"kdeglobals": "[General]\nColorScheme=ProteaDark\n[KDE]\nLookAndFeelPackage=org.kde.breezedark.desktop\n",
		"plasmarc":   "[Theme]\nname=protea-dark\n",
	})
	if err := r.RevertCore(); err != nil {
		t.Fatal(err)
	}
	check(t, ran, "plasma-apply-colorscheme BreezeDark", "plasma-apply-desktoptheme default")
}

func TestRevertLeavesOtherChoicesAlone(t *testing.T) {
	r, ran := fakeSetup(t, map[string]string{
		"kdeglobals": "[General]\nColorScheme=Nordic\n[KDE]\nwidgetStyle=Oxygen\n",
		"kwinrc":     "[org.kde.kdecoration2]\ntheme=__aurorae__svg__ChromeOS-dark\n",
	})
	if err := errors.Join(r.RevertCore(), r.RevertComponents()); err != nil {
		t.Fatal(err)
	}
	check(t, ran)
}

func TestRevertComponents(t *testing.T) {
	r, ran := fakeSetup(t, map[string]string{
		"kdeglobals": "[KDE]\nwidgetStyle=Protea\n",
		"kwinrc":     "[org.kde.kdecoration2]\nlibrary=org.proteadesktop.protea\ntheme=Protea\n",
	})
	if err := r.RevertComponents(); err != nil {
		t.Fatal(err)
	}
	check(t, ran,
		"kwriteconfig6 --file kdeglobals --group KDE --key widgetStyle Breeze",
		"dbus-send --session --type=signal /KGlobalSettings org.kde.KGlobalSettings.notifyChange int32:2 int32:0",
		"kwin-applywindowdecoration Breeze")
}

func TestInteractiveCommands(t *testing.T) {
	dir := t.TempDir()
	cmd := (&Runner{DryRun: true}).BuildCommand(dir)
	cmd.Stdin = strings.NewReader("\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Dry run, would run: makepkg --cleanbuild --install", "in " + dir} {
		if !strings.Contains(string(out), want) {
			t.Errorf("dry-run build output missing %q:\n%s", want, out)
		}
	}

	// A failing step's exit status reaches the installer, after the pause.
	cmd = (&Runner{}).Interactive(true, "", nil, "false")
	cmd.Stdin = strings.NewReader("\n")
	if err := cmd.Run(); err == nil {
		t.Error("failing command reported success")
	}
	cmd = (&Runner{}).Interactive(false, "", nil, "true")
	if err := cmd.Run(); err != nil {
		t.Errorf("succeeding command reported %v", err)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, _ = fakeSetup(t, map[string]string{"kdeglobals": "[General]\nColorScheme=ProteaDark\n"})
	r := &Runner{DryRun: true}
	if err := errors.Join(r.InstallCore(repoFiles), r.RevertCore()); err != nil {
		t.Fatal(err)
	}
	if coreInstalled() {
		t.Error("dry run installed files")
	}
	log := strings.Join(r.TakeLog(), "\n")
	for _, want := range []string{"would copy theme/color-schemes/ProteaDark.colors", "would run: plasma-apply-colorscheme BreezeDark"} {
		if !strings.Contains(log, want) {
			t.Errorf("dry-run log missing %q:\n%s", want, log)
		}
	}
}
