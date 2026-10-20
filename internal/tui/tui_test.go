package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"proteadesktop.org/protea-theme/internal/system"
)

// repoFiles are this repo's files, which the installer embeds.
var repoFiles = os.DirFS("../..")

// start returns a dry-run model showing status, after its first refresh.
func start(t *testing.T, status system.Status) *Model {
	t.Helper()
	m := New(repoFiles, &system.Runner{DryRun: true}, func() system.Status { return status })
	drain(m, m.Init())
	return m
}

// drain runs cmd and everything it leads to, except spinner frames and
// terminal hand-overs, which need a real program.
func drain(m *Model, cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case spinner.TickMsg, tea.QuitMsg:
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
}

func press(m *Model, keys ...string) {
	codes := map[string]rune{"enter": tea.KeyEnter, "down": tea.KeyDown, "up": tea.KeyUp, "esc": tea.KeyEscape, "space": tea.KeySpace}
	for _, k := range keys {
		msg := tea.KeyPressMsg{Code: codes[k]}
		if msg.Code == 0 {
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		_, cmd := m.Update(msg)
		drain(m, cmd)
	}
}

// choose moves to the row with label and presses enter.
func choose(t *testing.T, m *Model, label string) {
	t.Helper()
	moveTo(t, m, label)
	press(m, "enter")
}

// tick moves to the checkbox row with label and toggles it.
func tick(t *testing.T, m *Model, label string) {
	t.Helper()
	moveTo(t, m, label)
	press(m, "space")
}

func moveTo(t *testing.T, m *Model, label string) {
	t.Helper()
	for i, r := range m.rows() {
		if r.label == label {
			m.cursor = i
			return
		}
	}
	t.Fatalf("no row %q on screen:\n%s", label, screen(m))
}

func labels(m *Model) []string {
	var out []string
	for _, r := range m.rows() {
		out = append(out, r.label)
	}
	return out
}

func screen(m *Model) string { return ansi.Strip(m.View().Content) }

// flat collapses whitespace, so text matches however the screen wraps it.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

func expect(t *testing.T, m *Model, want ...string) {
	t.Helper()
	s := screen(m)
	for _, w := range want {
		if !strings.Contains(flat(s), flat(w)) {
			t.Errorf("screen missing %q:\n%s", w, s)
		}
	}
}

func expectNot(t *testing.T, m *Model, unwanted ...string) {
	t.Helper()
	s := screen(m)
	for _, w := range unwanted {
		if strings.Contains(flat(s), flat(w)) {
			t.Errorf("screen shouldn't show %q:\n%s", w, s)
		}
	}
}

var (
	fresh = system.Status{Plasma: "6.8.0", Kdecoration: "6.8.0-1"}

	// The theme in use as a Global Theme, plus both components built for
	// Plasma 6.8 and in use.
	everything = system.Status{
		Plasma: "6.8.0", Kdecoration: "6.8.0-1",
		CoreInstalled: true, GlobalThemeActive: true, ColorsActive: true, PlasmaStyleActive: true,
		Package:     "0.1.0-1",
		Build:       map[string]string{"WITH_STYLE": "ON", "WITH_DECORATIONS": "ON", "KDECORATION_VERSION": "6.8.0", "QT_VERSION": "6.12.0"},
		StyleActive: true, DecorationActive: true,
	}
)

func TestWelcome(t *testing.T) {
	m := start(t, fresh)
	expect(t, m, "● Welcome", "○ Theme", "Dry run", "Plasma 6.8.0 ✓", "Protea Dark not installed", "› Install Protea Dark")
	if got := labels(m); len(got) != 1 {
		t.Errorf("nothing installed should offer only installing, got %q", got)
	}

	m = start(t, everything)
	expect(t, m, "installed · in use: Global Theme, colors, Plasma Style",
		"application style and window decoration · in use: application style, window decoration")
	if got := labels(m); got[0] != "Update or change Protea Dark" || got[len(got)-1] != "Uninstall" {
		t.Errorf("rows = %q, want changing first and Uninstall last", got)
	}

	st := everything
	st.KdecorationUpgrade = "6.9.0-1"
	m = start(t, st)
	expect(t, m, "Plasma 6.9 is available")
	if got := labels(m); strings.Join(got, "|") != "Update or change Protea Dark|Prepare for Plasma 6.9|Uninstall" {
		t.Errorf("rows = %q", got)
	}

	m = start(t, system.Status{Plasma: "6.9.0"})
	expect(t, m, "Protea is made for Plasma 6.8. On Plasma 6.9")
}

func TestInstallDefaults(t *testing.T) {
	m := start(t, fresh)
	press(m, "enter")
	expect(t, m, "✓ Welcome", "● Theme", "› (•) Full Global Theme", "( ) Colors and Plasma Style", "( ) Install only")
	press(m, "enter")
	expect(t, m, "● Components", "[ ] Application style", "[ ] Window decoration", "[x] Use them once installed")
	expectNot(t, m, "Build again", "pinned")
	press(m, "enter")
	expect(t, m, "● Review", "1 Copy Protea Dark into", "2 Apply the Protea Dark Global Theme", "› Install", "saved first")
	expectNot(t, m, "makepkg", "needs sudo")
}

func TestBackKeepsChoices(t *testing.T) {
	m := start(t, fresh)
	press(m, "enter", "down", "enter")
	tick(t, m, "Window decoration")
	press(m, "esc")
	expect(t, m, "(•) Colors and Plasma Style")
	press(m, "enter")
	expect(t, m, "[x] Window decoration", "pinned to Plasma 6.8")
	press(m, "esc", "esc")
	expect(t, m, "● Welcome")
}

func TestInstallWithComponents(t *testing.T) {
	st := fresh
	st.MissingTools = []string{"cmake"}
	m := start(t, st)
	press(m, "enter", "enter")
	tick(t, m, "Application style")
	expect(t, m, "Missing build tools (cmake) will be installed first.")
	press(m, "enter")
	expect(t, m,
		"1 Install the build tools: cmake needs sudo",
		"2 Copy Protea Dark into",
		"3 Apply the Protea Dark Global Theme",
		"4 Build and install the application style with makepkg needs sudo",
		"5 Switch the application style to Protea",
		"take over the terminal")
	expectNot(t, m, "temporary")
}

func TestRunInstallDryRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "kdeglobals"), []byte("[General]\nColorScheme=Nordic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := start(t, fresh)
	press(m, "enter", "down", "enter", "enter")
	choose(t, m, "Install")
	expect(t, m, "✓ Review", "✓ Install",
		"✓ Copy Protea Dark into", "would copy theme/color-schemes/ProteaDark.colors",
		"✓ Switch colors and Plasma Style to Protea Dark",
		"would run: plasma-apply-colorscheme ProteaDark", "would run: plasma-apply-desktoptheme protea-dark",
		"Done.", "enter start over")
	press(m, "enter")
	expect(t, m, "● Welcome", "○ Theme")
}

func TestChangingExistingInstall(t *testing.T) {
	m := start(t, everything)
	press(m, "enter", "enter")
	expect(t, m, "[x] Application style", "[x] Window decoration", "[x] Use them once installed", "[ ] Build again",
		"Installed: protea 0.1.0-1, Qt 6.12.0, Plasma 6.8.")
	press(m, "enter")
	// Nothing to rebuild or switch: only the theme files are refreshed.
	expect(t, m, "1 Update Protea Dark in")
	expectNot(t, m, "2 ", "makepkg", "Switch")

	press(m, "esc")
	tick(t, m, "Build again")
	press(m, "enter")
	expect(t, m, "Build and install the application style and window decoration with makepkg")

	press(m, "esc")
	tick(t, m, "Build again")
	tick(t, m, "Window decoration")
	press(m, "enter")
	expect(t, m, "Switch back to your previous window decoration", "Build and install the application style with makepkg")
	expectNot(t, m, "Switch the window decoration to Protea")

	press(m, "esc")
	tick(t, m, "Application style")
	press(m, "enter")
	expect(t, m, "Switch back to your previous application style", "Remove the protea package with pacman needs sudo")
	expectNot(t, m, "makepkg")
}

func TestColorsOverGlobalThemeKeepsComponents(t *testing.T) {
	st := everything
	st.GlobalThemeActive = false
	m := start(t, st)
	press(m, "enter")
	expect(t, m, "(•) Colors and Plasma Style")
	press(m, "up", "enter", "enter")
	// The Global Theme resets style and decoration, so they're switched to
	// Protea again afterwards.
	expect(t, m, "Apply the Protea Dark Global Theme", "Switch the application style to Protea", "Switch the window decoration to Protea")
}

func TestMismatchForcesRebuild(t *testing.T) {
	st := everything
	st.Kdecoration = "6.9.0-1"
	m := start(t, st)
	expect(t, m, "built for Plasma 6.8, but Plasma 6.9 is installed")
	press(m, "enter", "enter", "enter")
	expect(t, m, "Build and install the application style and window decoration with makepkg")
}

func TestUninstall(t *testing.T) {
	m := start(t, everything)
	choose(t, m, "Uninstall")
	expect(t, m, "● Remove", "[x] Protea Dark theme", "[x] Application style and window decoration")
	press(m, "enter")
	expect(t, m, "● Review",
		"1 Switch back to your previous application style and window decoration",
		"2 Remove the protea package with pacman needs sudo",
		"3 Switch back to your previous theme settings",
		"4 Remove the theme files",
		"› Uninstall")

	press(m, "esc")
	tick(t, m, "Application style and window decoration")
	press(m, "enter")
	expectNot(t, m, "pacman")

	press(m, "esc")
	tick(t, m, "Protea Dark theme")
	press(m, "enter")
	expect(t, m, "Nothing to change.", "› Back")
	press(m, "enter")
	expect(t, m, "● Remove")
}

func TestPrepareForUpgrade(t *testing.T) {
	st := everything
	st.KdecorationUpgrade = "6.9.0-1"
	m := start(t, st)
	choose(t, m, "Prepare for Plasma 6.9")
	expect(t, m, "✓ Welcome ─ ● Review ─ ○ Prepare",
		"1 Switch back to your previous application style and window decoration",
		"2 Remove the protea package with pacman needs sudo",
		"run sudo pacman -Syu", "› Prepare")
}
