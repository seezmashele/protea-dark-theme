package tui

import (
	"os"
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

	// Protea Dark applied as the Global Theme, plus the C++ components
	// built for Plasma 6.8 and in use.
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
	expect(t, m, "● Welcome", "○ Options", "Dry run",
		"▍Welcome", "Protea Dark is a dark theme for KDE Plasma, forked from Breeze.",
		"Plasma 6.8.0 ✓", "Protea Dark not installed",
		"› Install Protea Dark", "Uninstall · nothing installed")
	press(m, "down")
	if m.cursor != 0 {
		t.Errorf("cursor moved onto the disabled Uninstall row")
	}
	press(m, "enter")
	expect(t, m, "● Options")

	m = start(t, everything)
	expect(t, m, "installed · in use: Global Theme, colors, Plasma Style",
		"installed · in use: application style, window decoration")
	expectNot(t, m, "nothing installed")
	press(m, "down")
	if got := m.rows()[m.cursor].label; got != "Uninstall" {
		t.Errorf("down from Update selected %q, want Uninstall", got)
	}

	st := everything
	st.KdecorationUpgrade = "6.9.0-1"
	m = start(t, st)
	expect(t, m, "Plasma 6.9 is available")
	if got := strings.Join(labels(m), "|"); got != "Update Protea Dark|Prepare for Plasma 6.9|Uninstall" {
		t.Errorf("rows = %q", got)
	}

	m = start(t, system.Status{Plasma: "6.9.0"})
	expect(t, m, "Protea is made for Plasma 6.8. On Plasma 6.9")
}

func TestInstallDefaults(t *testing.T) {
	m := start(t, fresh)
	press(m, "enter")
	expect(t, m, "✓ Welcome", "● Options", "▍Choose what to install",
		"Protea Dark's Global Theme, Plasma Style and color scheme are always installed into", "The C++ components are optional.",
		"› [ ] C++ components")
	expectNot(t, m, "Build again", "pinned", "Application style", "Use them")
	press(m, "enter")
	expect(t, m, "● Review", "▍Review the plan", "Nothing has changed yet. These steps run when you choose Install.",
		"1 Install Protea Dark into", "› Install",
		"Your current look doesn't change. Apply Protea Dark yourself in System Settings afterwards.")
	expectNot(t, m, "2 ", "makepkg", "needs sudo", "Apply the")
}

func TestBackKeepsChoices(t *testing.T) {
	m := start(t, fresh)
	press(m, "enter")
	tick(t, m, "C++ components")
	press(m, "enter", "esc")
	expect(t, m, "[x] C++ components", "pinned to Plasma 6.8")
	press(m, "esc")
	expect(t, m, "● Welcome")
}

func TestInstallWithComponents(t *testing.T) {
	st := fresh
	st.MissingTools = []string{"cmake"}
	m := start(t, st)
	press(m, "enter")
	tick(t, m, "C++ components")
	expect(t, m, "Missing build tools (cmake) will be installed first.")
	press(m, "enter")
	expect(t, m,
		"1 Install the build tools: cmake needs sudo",
		"2 Install Protea Dark into",
		"3 Build and install the C++ components with makepkg needs sudo")
	expectNot(t, m, "temporary", "4 ", "Switch")

	// The first step needs sudo: the terminal is handed over, which tests
	// can't do, so the Install page waits on it.
	choose(t, m, "Install")
	expect(t, m, "● Install", "▍Installing", "Steps that need sudo take over the terminal. Press Enter afterwards to come back here.")
}

func TestRunInstallDryRun(t *testing.T) {
	m := start(t, fresh)
	press(m, "enter", "enter")
	choose(t, m, "Install")
	expect(t, m, "✓ Review", "✓ Install",
		"✓ Install Protea Dark into", "would copy theme/color-schemes/ProteaDark.colors",
		"▍Installed", "Protea is ready. Apply it yourself in System Settings → Colors & Themes:",
		"Global Theme → Protea Dark", "Colors → Protea Dark", "Plasma Style → Protea Dark",
		"enter start over")
	expectNot(t, m, "Application Style")
	press(m, "enter")
	expect(t, m, "● Welcome", "○ Options")

	// With the C++ components installed, the reminder covers them too.
	m = start(t, everything)
	press(m, "enter", "enter")
	choose(t, m, "Install")
	expect(t, m, "Application Style → Protea", "Window Decorations → Protea")
}

func TestChangingExistingInstall(t *testing.T) {
	m := start(t, everything)
	press(m, "enter")
	expect(t, m, "[x] C++ components", "[ ] Build again", "Installed: protea 0.1.0-1, built for Qt 6.12.0 and Plasma 6.8.")
	press(m, "enter")
	// Already built for this Plasma: only the theme files are refreshed.
	expect(t, m, "1 Update Protea Dark in")
	expectNot(t, m, "2 ", "makepkg")

	press(m, "esc")
	tick(t, m, "Build again")
	press(m, "enter")
	expect(t, m, "2 Build and install the C++ components with makepkg")

	press(m, "esc")
	tick(t, m, "Build again")
	tick(t, m, "C++ components")
	press(m, "enter")
	expect(t, m,
		"2 Switch the application style and window decoration back to Breeze",
		"3 Remove the C++ components with pacman needs sudo")
	expectNot(t, m, "makepkg")
}

func TestMismatchForcesRebuild(t *testing.T) {
	st := everything
	st.Kdecoration = "6.9.0-1"
	m := start(t, st)
	expect(t, m, "built for Plasma 6.8, but Plasma 6.9 is installed")
	press(m, "enter", "enter")
	expect(t, m, "Build and install the C++ components with makepkg")
}

func TestUninstall(t *testing.T) {
	m := start(t, everything)
	choose(t, m, "Uninstall")
	expect(t, m, "● Remove", "▍Choose what to remove", "switched to Breeze Dark first", "[x] Protea Dark theme", "[x] C++ components")
	press(m, "enter")
	expect(t, m, "● Review",
		"1 Switch the application style and window decoration back to Breeze",
		"2 Remove the C++ components with pacman needs sudo",
		"3 Switch to the Breeze Dark Global Theme",
		"4 Remove the theme files",
		"These steps run when you choose Uninstall.", "› Uninstall")

	press(m, "esc")
	tick(t, m, "C++ components")
	tick(t, m, "Protea Dark theme")
	press(m, "enter")
	expect(t, m, "Nothing to change.", "› Back")
	press(m, "enter")
	expect(t, m, "● Remove")

	// Only Protea's colors picked, not the whole Global Theme.
	st := system.Status{Plasma: "6.8.0", CoreInstalled: true, ColorsActive: true}
	m = start(t, st)
	choose(t, m, "Uninstall")
	press(m, "enter")
	expect(t, m, "1 Switch colors and Plasma Style back to Breeze Dark's", "2 Remove the theme files")
	expectNot(t, m, "Global Theme", "pacman")
}

func TestPrepareForUpgrade(t *testing.T) {
	st := everything
	st.KdecorationUpgrade = "6.9.0-1"
	m := start(t, st)
	choose(t, m, "Prepare for Plasma 6.9")
	expect(t, m, "✓ Welcome ─ ● Review ─ ○ Prepare",
		"1 Switch the application style and window decoration back to Breeze",
		"2 Remove the C++ components with pacman needs sudo",
		"run sudo pacman -Syu", "› Prepare")
}

func TestLogo(t *testing.T) {
	want := strings.Join([]string{
		"█▀▀▄ █▀▀▄ ▄▀▀▄ ▀█▀ █▀▀▀ ▄▀▀▄    ▀█▀ █  █ █▀▀▀ █▄ ▄█ █▀▀▀",
		"█▀▀  █▀█  █  █  █  █▀▀  █▀▀█     █  █▀▀█ █▀▀  █ ▀ █ █▀▀ ",
		"▀    ▀  ▀  ▀▀   ▀  ▀▀▀▀ ▀  ▀     ▀  ▀  ▀ ▀▀▀▀ ▀   ▀ ▀▀▀▀",
	}, "\n")
	if got := ansi.Strip(logo); got != want {
		t.Errorf("logo:\n%s\nwant:\n%s", got, want)
	}
}
