package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	protea "proteadesktop.org/protea-theme"
	"proteadesktop.org/protea-theme/installer/internal/system"
)

// start returns a dry-run model showing status, after its first refresh.
func start(t *testing.T, status system.Status) *Model {
	t.Helper()
	m := New(protea.Files, &system.Runner{DryRun: true}, func() system.Status { return status })
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

// choose moves to the menu item with label and selects it.
func choose(t *testing.T, m *Model, label string) {
	t.Helper()
	for i, it := range m.items() {
		if it.label == label {
			m.cursor = i
			press(m, "enter")
			return
		}
	}
	t.Fatalf("no menu item %q on screen:\n%s", label, screen(m))
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

var pinnedPackage = system.Status{
	Plasma:      "6.8.0",
	Package:     "0.1.0-1",
	Build:       map[string]string{"WITH_STYLE": "ON", "WITH_DECORATIONS": "ON", "KDECORATION_VERSION": "6.8.0", "QT_VERSION": "6.12.0"},
	Kdecoration: "6.8.0-1",
}

func TestHomeShowsStatus(t *testing.T) {
	m := start(t, system.Status{Plasma: "6.8.0", CoreInstalled: true, ColorsActive: true, PlasmaStyleActive: true})
	expect(t, m, "dry run", "6.8.0", "installed · in use: colors, Plasma Style", "not installed", "› Theme")
	expectNot(t, m, "made for Plasma")

	m = start(t, system.Status{Plasma: "6.9.0"})
	expect(t, m, "Protea is made for Plasma 6.8. On Plasma 6.9")
}

func TestNavigation(t *testing.T) {
	m := start(t, system.Status{Plasma: "6.8.0"})
	press(m, "down", "enter")
	expect(t, m, "Optional components", "Build tools", "[x] Application style")
	press(m, "esc")
	expect(t, m, "› Theme")
	press(m, "enter")
	// Only installing is offered until the theme is installed.
	expect(t, m, "Install Protea Dark")
	expectNot(t, m, "Use Protea colors", "Uninstall")
}

func TestApplyColorsDryRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "kdeglobals"), []byte("[General]\nColorScheme=Nordic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := start(t, system.Status{Plasma: "6.8.0", CoreInstalled: true})
	press(m, "enter")
	choose(t, m, "Use Protea colors and Plasma Style")
	expect(t, m, "✓ Switch the color scheme and Plasma Style to Protea Dark",
		"would run: plasma-apply-colorscheme ProteaDark", "would run: plasma-apply-desktoptheme protea-dark", "Done.")
	press(m, "enter")
	expect(t, m, "Use the full Protea Dark Global Theme")
}

func TestBuildChecksFirst(t *testing.T) {
	m := start(t, system.Status{Plasma: "6.8.0", MissingTools: []string{"cmake"}})
	press(m, "down", "enter")
	expect(t, m, "missing cmake", "Install build tools")
	choose(t, m, "Build and install")
	expect(t, m, "Install the build tools first: cmake.", "Didn't finish.")

	m = start(t, system.Status{Plasma: "6.8.0"})
	press(m, "down", "enter")
	press(m, "space", "down", "space")
	expect(t, m, "[ ] Application style", "[ ] Window decoration")
	choose(t, m, "Build and install")
	expect(t, m, "Select the application style, the window decoration or both first.")
}

func TestPinnedPackage(t *testing.T) {
	m := start(t, pinnedPackage)
	press(m, "down", "enter")
	expect(t, m, "protea 0.1.0-1 · application style + window decoration", "Plasma 6.8 · Qt 6.12.0",
		"pinned to Plasma 6.8", "Remove the package")
	expectNot(t, m, "Prepare for Plasma")
}

func TestUpgradeBlockedOffersPrepare(t *testing.T) {
	st := pinnedPackage
	st.KdecorationUpgrade = "6.9.0-1"
	m := start(t, st)
	press(m, "down", "enter")
	expect(t, m, "Plasma 6.9 is available", "Prepare for Plasma 6.9")
	expectNot(t, m, "Remove the package")
}

func TestUseRefusesMismatchedDecoration(t *testing.T) {
	st := pinnedPackage
	st.Kdecoration = "6.9.0-1"
	m := start(t, st)
	press(m, "down", "enter")
	expect(t, m, "built for Plasma 6.8, but Plasma 6.9 is installed")
	choose(t, m, "Use Protea for the selected components")
	expect(t, m, "The window decoration was built for Plasma 6.8, but Plasma 6.9 is installed. Build it again first.")
}
