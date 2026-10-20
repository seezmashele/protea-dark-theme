package system

import (
	"errors"
	"io/fs"
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
	if err := r.InstallCore(repoFiles, DefaultAccent); err != nil {
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
	if err := r.InstallCore(repoFiles, DefaultAccent); err != nil {
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
	if err := errors.Join(r.InstallCore(repoFiles, DefaultAccent), r.RevertCore()); err != nil {
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

// groups parses a KDE color file into group → key → value.
func groups(text string) map[string]map[string]string {
	out := map[string]map[string]string{}
	group := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "[") {
			group = line
			out[group] = map[string]string{}
		} else if k, v, ok := strings.Cut(line, "="); ok && group != "" {
			out[group][k] = v
		}
	}
	return out
}

func readRepo(t *testing.T, path string) string {
	t.Helper()
	data, err := fs.ReadFile(repoFiles, path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func apply(t *testing.T, text, accent string) string {
	t.Helper()
	out, err := ApplyAccent(text, accent)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestApplyAccentToColorScheme(t *testing.T) {
	// The repo's scheme has the default accent.
	teal := readRepo(t, "theme/color-schemes/ProteaDark.colors")
	if apply(t, teal, DefaultAccent) != teal {
		t.Error("the repo's color scheme doesn't have the default accent, or applying it twice changes the file")
	}
	for _, breeze := range []string{"61,174,233", "30,87,116"} {
		if strings.Contains(teal, "="+breeze+"\n") {
			t.Errorf("Breeze's accent %s is still in the scheme", breeze)
		}
	}

	pink := apply(t, teal, "#e0577f")
	g := groups(pink)
	for name, keys := range g {
		if !strings.HasPrefix(name, "[Colors:") {
			continue
		}
		for _, k := range []string{"DecorationFocus", "DecorationHover"} {
			if keys[k] != "224,87,127" {
				t.Errorf("%s %s = %s, want the accent", name, k, keys[k])
			}
		}
		if link, ok := keys["ForegroundLink"]; ok && name != "[Colors:Selection]" {
			l, _ := parseKDE(link)
			bg, _ := parseKDE(keys["BackgroundNormal"])
			if c := contrast(l, bg); c < 5 {
				t.Errorf("%s link contrast %.1f, want at least 5", name, c)
			}
		}
	}
	sel := g["[Colors:Selection]"]
	selBg, _ := parseKDE(sel["BackgroundNormal"])
	selFg, _ := parseKDE(sel["ForegroundNormal"])
	if c := contrast(selBg, selFg); c < 4.5 {
		t.Errorf("selection text contrast %.1f, want at least 4.5", c)
	}

	// Only accent keys change: the other lines are untouched.
	before, after := strings.Split(teal, "\n"), strings.Split(pink, "\n")
	if len(before) != len(after) {
		t.Fatalf("line count changed: %d → %d", len(before), len(after))
	}
	accentKeys := []string{"DecorationFocus", "DecorationHover", "ForegroundActive", "ForegroundLink",
		"BackgroundNormal", "BackgroundAlternate", "ForegroundNormal", "ForegroundInactive"}
	changed := 0
	for i := range before {
		if before[i] != after[i] {
			changed++
			if k, _, _ := strings.Cut(before[i], "="); !slices.Contains(accentKeys, k) {
				t.Errorf("unexpected change: %q → %q", before[i], after[i])
			}
		}
	}
	if changed == 0 {
		t.Error("changing the accent changed nothing")
	}

	// Switching back leaves no trace of the other accent.
	if apply(t, pink, DefaultAccent) != teal {
		t.Error("switching accents leaves traces of the previous one")
	}
}

func TestApplyAccentToPlasmaStyle(t *testing.T) {
	original := readRepo(t, "theme/desktoptheme/protea-dark/colors")
	teal := apply(t, original, "#5ba9ae")
	if strings.Contains(teal, "DecorationFocus") || strings.Contains(teal, "[Colors:Selection]\nBackgroundNormal") {
		t.Error("keys the Plasma Style leaves out (to follow the system colors) were added")
	}
	if strings.Contains(teal, "=30,87,116\n") {
		t.Error("Breeze's darker accent is still in the Plasma Style")
	}
	if g := groups(teal); g["[Colors:Selection]"]["ForegroundLink"] != groups(original)["[Colors:Selection]"]["ForegroundLink"] {
		t.Error("the link color on selections changed")
	}
}

func TestAccentPresetsAndPreview(t *testing.T) {
	if DefaultAccent != "#5ba9ae" || AccentName("#5BA9AE") != "Teal" || AccentName("#123456") != "#123456" {
		t.Errorf("default %s, names %q %q", DefaultAccent, AccentName("#5BA9AE"), AccentName("#123456"))
	}
	for _, a := range Accents {
		sel, fg, link, err := AccentPreview(a.Hex)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := parseHex(sel)
		f, _ := parseHex(fg)
		l, _ := parseHex(link)
		if contrast(s, f) < 4.5 || contrast(l, viewBackground) < 5 {
			t.Errorf("%s: unreadable selection (%s on %s) or link (%s)", a.Name, fg, sel, link)
		}
	}
	if _, err := ApplyAccent("", "teal"); err == nil {
		t.Error("a bad color was accepted")
	}
}

func TestInstallWritesAccent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	r := &Runner{}
	if err := r.InstallCore(repoFiles, "#5ba9ae"); err != nil {
		t.Fatal(err)
	}
	if got := installedAccent(); got != "#5ba9ae" {
		t.Errorf("installed accent %q, want #5ba9ae", got)
	}
	if err := r.InstallCore(repoFiles, "#e0577f"); err != nil {
		t.Fatal(err)
	}
	if got := installedAccent(); got != "#e0577f" {
		t.Errorf("installed accent after changing it: %q, want #e0577f", got)
	}
}
