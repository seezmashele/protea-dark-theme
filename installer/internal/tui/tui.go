// Package tui is the installer's terminal interface.
package tui

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"proteadesktop.org/protea-theme/installer/internal/system"
)

type page int

const (
	pageHome page = iota
	pageTheme
	pageComponents
	pageProgress
	pageSummary
)

// item is one row of a page's menu.
type item struct {
	label, hint string
	toggle      *bool          // set for checkbox rows
	action      func() tea.Cmd // nil for checkbox rows
}

type stepState int

const (
	stepPending stepState = iota
	stepRunning
	stepDone
	stepFailed
	stepSkipped
)

// step is one part of a task. Quick steps run in the background; steps
// that need the terminal (sudo, makepkg) run through tea.ExecProcess.
type step struct {
	name   string
	run    func() error
	exec   func() *exec.Cmd
	always bool // run even after an earlier step failed (cleanup)

	state stepState
	err   error
	log   []string // dry-run messages
}

type statusMsg system.Status

type stepDoneMsg struct {
	index int
	err   error
	log   []string
}

// Model is the installer's state.
type Model struct {
	files      fs.FS
	runner     *system.Runner
	readStatus func() system.Status

	status system.Status
	loaded bool
	width  int

	page   page
	cursor int

	wantStyle      bool
	wantDecoration bool

	task    string
	steps   []step
	failed  bool
	note    string // shown in the summary after a successful task
	reason  string // why a task couldn't start
	back    page   // where the summary returns to
	spinner spinner.Model
}

// New returns the installer model. readStatus inspects the system; tests
// pass a fake.
func New(files fs.FS, runner *system.Runner, readStatus func() system.Status) *Model {
	return &Model{
		files:          files,
		runner:         runner,
		readStatus:     readStatus,
		wantStyle:      true,
		wantDecoration: true,
		spinner:        spinner.New(spinner.WithSpinner(dot()), spinner.WithStyle(accent)),
	}
}

// dot is spinner.Dot without the space after each frame, so running steps
// line up with finished ones.
func dot() spinner.Spinner {
	s := spinner.Dot
	s.Frames = make([]string, len(spinner.Dot.Frames))
	for i, f := range spinner.Dot.Frames {
		s.Frames[i] = strings.TrimSpace(f)
	}
	return s
}

var (
	accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0457B"))
	title   = accent.Bold(true)
	bold    = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	good    = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FB878"))
	bad     = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5534B"))
	warning = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0A33B"))
)

func (m *Model) Init() tea.Cmd { return m.refresh() }

func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg { return statusMsg(m.readStatus()) }
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status, m.loaded = system.Status(msg), true
		m.cursor = min(m.cursor, max(len(m.items())-1, 0))
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case spinner.TickMsg:
		if m.page == pageProgress {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case stepDoneMsg:
		return m, m.finishStep(msg)
	case tea.KeyPressMsg:
		return m, m.key(msg.String())
	}
	return m, nil
}

func (m *Model) key(k string) tea.Cmd {
	if k == "ctrl+c" {
		return tea.Quit
	}
	switch m.page {
	case pageProgress:
		return nil
	case pageSummary:
		switch k {
		case "q":
			return tea.Quit
		case "enter", "esc", "backspace", "space":
			m.open(m.back)
		}
		return nil
	}
	if !m.loaded {
		return nil
	}
	items := m.items()
	switch k {
	case "q":
		return tea.Quit
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, len(items)-1)
	case "esc", "backspace", "left", "h":
		if m.page != pageHome {
			m.open(pageHome)
		}
	case "enter", "space", "right", "l":
		it := items[m.cursor]
		if it.toggle != nil {
			*it.toggle = !*it.toggle
			return nil
		}
		return it.action()
	}
	return nil
}

func (m *Model) open(p page) {
	m.page, m.cursor = p, 0
}

func (m *Model) goTo(p page) func() tea.Cmd {
	return func() tea.Cmd { m.open(p); return nil }
}

// start runs steps as a task, showing progress and then a summary.
func (m *Model) start(task, note string, steps ...step) tea.Cmd {
	m.task, m.note, m.reason, m.steps, m.failed = task, note, "", steps, false
	m.back, m.page = m.page, pageProgress
	return tea.Batch(m.spinner.Tick, m.runStep(0))
}

// fail shows a summary for a task that can't start, with the reason.
func (m *Model) fail(task, reason string) tea.Cmd {
	m.task, m.note, m.reason, m.steps, m.failed = task, "", reason, nil, true
	m.back, m.page = m.page, pageSummary
	return nil
}

func (m *Model) runStep(i int) tea.Cmd {
	s := &m.steps[i]
	s.state = stepRunning
	done := func(err error) tea.Msg { return stepDoneMsg{index: i, err: err, log: m.runner.TakeLog()} }
	if s.exec != nil {
		return tea.ExecProcess(s.exec(), done)
	}
	run := s.run
	return func() tea.Msg { return done(run()) }
}

func (m *Model) finishStep(msg stepDoneMsg) tea.Cmd {
	s := &m.steps[msg.index]
	s.log = msg.log
	if msg.err != nil {
		s.state, s.err, m.failed = stepFailed, msg.err, true
	} else {
		s.state = stepDone
	}
	next := msg.index + 1
	for next < len(m.steps) && m.failed && !m.steps[next].always {
		m.steps[next].state = stepSkipped
		next++
	}
	if next < len(m.steps) {
		return m.runStep(next)
	}
	m.page = pageSummary
	return m.refresh()
}

// tildePath shortens a path in the home folder to ~/...
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+"/") {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func (m *Model) items() []item {
	st := m.status
	switch m.page {
	case pageHome:
		return []item{
			{label: "Theme", hint: "Install, use or remove Protea Dark's colors, Plasma Style and Global Theme.", action: m.goTo(pageTheme)},
			{label: "Optional components", hint: "Build Protea's application style and window decoration from source.", action: m.goTo(pageComponents)},
			{label: "Quit", action: func() tea.Cmd { return tea.Quit }},
		}
	case pageTheme:
		return m.themeItems(st)
	case pageComponents:
		return m.componentItems(st)
	}
	return nil
}

func (m *Model) themeItems(st system.Status) []item {
	install := item{
		label: "Install Protea Dark",
		hint:  "Copies the Global Theme, Plasma Style and color scheme into " + tildePath(system.DataHome()) + ". Doesn't change your desktop.",
	}
	if st.CoreInstalled {
		install.label = "Update the theme files"
		install.hint = "Replaces the installed theme files with this installer's version."
	}
	install.action = func() tea.Cmd {
		return m.start(install.label, "", step{name: "Copy the theme files", run: func() error { return m.runner.InstallCore(m.files) }})
	}
	items := []item{install}
	if st.CoreInstalled {
		items = append(items,
			item{
				label: "Use Protea colors and Plasma Style",
				hint:  "Keeps your icons, cursors, application style and window decoration.",
				action: func() tea.Cmd {
					return m.start("Use Protea colors and Plasma Style", "",
						step{name: "Switch the color scheme and Plasma Style to Protea Dark", run: m.runner.ApplyColors})
				},
			},
			item{
				label: "Use the full Protea Dark Global Theme",
				hint:  "Also switches icons, cursors, application style and window decoration to Breeze's. Uninstalling switches them back.",
				action: func() tea.Cmd {
					return m.start("Use the full Protea Dark Global Theme", "",
						step{name: "Apply the Protea Dark Global Theme", run: m.runner.ApplyGlobalTheme})
				},
			},
			item{
				label: "Uninstall Protea Dark",
				hint:  "Switches back to your previous colors, Plasma Style and Global Theme settings, then removes the theme files.",
				action: func() tea.Cmd {
					return m.start("Uninstall Protea Dark", "",
						step{name: "Switch back to your previous theme settings", run: m.runner.RevertCore},
						step{name: "Remove the theme files", run: m.runner.UninstallCore})
				},
			},
		)
	}
	return append(items, item{label: "Back", action: m.goTo(pageHome)})
}

func (m *Model) componentItems(st system.Status) []item {
	items := []item{
		{label: "Application style", hint: "Protea's look for buttons, menus, scrollbars and other controls in Qt apps.", toggle: &m.wantStyle},
		{label: "Window decoration", hint: "Protea's title bars and window borders, drawn by KWin.", toggle: &m.wantDecoration},
		{label: "Build and install", hint: "Compiles the selected components with makepkg and installs the protea package. Asks for your sudo password.", action: m.build},
	}
	if st.Package != "" {
		items = append(items, item{label: "Use Protea for the selected components", hint: "Switches your application style and/or window decoration to Protea.", action: m.use})
	}
	if st.StyleActive || st.DecorationActive {
		items = append(items, item{label: "Switch back", hint: "Restores the application style and window decoration you had before Protea.", action: func() tea.Cmd {
			return m.start("Switch back", "", m.revertStep())
		}})
	}
	if st.UpgradeBlocked() {
		next := system.Feature(st.KdecorationUpgrade)
		items = append(items, item{
			label: "Prepare for Plasma " + next,
			hint:  "Switches back from Protea and removes the package, so pacman can upgrade Plasma. Build again after upgrading.",
			action: func() tea.Cmd {
				return m.remove("Prepare for Plasma "+next, "Now run `sudo pacman -Syu`, then come back and build Protea again.")
			},
		})
	} else if st.Package != "" {
		items = append(items, item{label: "Remove the package", hint: "Switches back from Protea first, then runs pacman -R. Asks for your sudo password.", action: func() tea.Cmd {
			return m.remove("Remove the package", "")
		}})
	}
	if len(st.MissingTools) > 0 {
		items = append(items, item{label: "Install build tools", hint: "Runs sudo pacman -S --needed " + strings.Join(st.MissingTools, " ") + ".", action: func() tea.Cmd {
			return m.start("Install build tools", "", step{name: "Install " + strings.Join(st.MissingTools, ", ") + " with pacman", exec: m.runner.ToolsCommand})
		}})
	}
	return append(items, item{label: "Back", action: m.goTo(pageHome)})
}

func (m *Model) revertStep() step {
	return step{name: "Switch back to your previous application style and window decoration", run: m.runner.RevertComponents}
}

func (m *Model) build() tea.Cmd {
	const task = "Build and install"
	if !m.wantStyle && !m.wantDecoration {
		return m.fail(task, "Select the application style, the window decoration or both first.")
	}
	if missing := m.status.MissingTools; len(missing) > 0 {
		return m.fail(task, "Install the build tools first: "+strings.Join(missing, ", ")+".")
	}
	var dir string
	style, decoration := m.wantStyle, m.wantDecoration
	return m.start(task, "Choose \"Use Protea for the selected components\" to switch to them.",
		step{name: "Copy the C++ sources to a temporary folder", run: func() (err error) {
			dir, err = m.runner.PrepareBuild(m.files)
			return err
		}},
		step{name: "Build and install with makepkg (asks for your sudo password)", exec: func() *exec.Cmd {
			return m.runner.BuildCommand(dir, style, decoration)
		}},
		step{name: "Remove the temporary build folder", always: true, run: func() error {
			if dir == "" {
				return nil
			}
			return m.runner.CleanBuild(dir)
		}},
	)
}

func (m *Model) use() tea.Cmd {
	const task = "Use Protea"
	st := m.status
	var steps []step
	if m.wantStyle {
		if !st.BuiltStyle() {
			return m.fail(task, "The installed package doesn't include the application style. Build it first.")
		}
		steps = append(steps, step{name: "Switch the application style to Protea", run: m.runner.UseStyle})
	}
	if m.wantDecoration {
		if !st.BuiltDecoration() {
			return m.fail(task, "The installed package doesn't include the window decoration. Build it first.")
		}
		if st.Mismatch() {
			return m.fail(task, fmt.Sprintf("The window decoration was built for Plasma %s, but Plasma %s is installed. Build it again first.",
				st.BuiltFor(), system.Feature(st.Kdecoration)))
		}
		steps = append(steps, step{name: "Switch the window decoration to Protea", run: m.runner.UseDecoration})
	}
	if len(steps) == 0 {
		return m.fail(task, "Select the application style, the window decoration or both first.")
	}
	return m.start(task, "", steps...)
}

func (m *Model) remove(task, note string) tea.Cmd {
	var steps []step
	if m.status.StyleActive || m.status.DecorationActive {
		steps = append(steps, m.revertStep())
	}
	steps = append(steps, step{name: "Remove the package with pacman (asks for your sudo password)", exec: func() *exec.Cmd {
		return m.runner.RemoveCommand(true)
	}})
	return m.start(task, note, steps...)
}

func (m *Model) View() tea.View {
	var b strings.Builder
	b.WriteString(title.Render("Protea Dark") + dim.Render("  installer"))
	if m.runner.DryRun {
		b.WriteString(warning.Render("  dry run: nothing will be changed"))
	}
	b.WriteString("\n\n")
	switch {
	case !m.loaded:
		b.WriteString(dim.Render("Checking your system…"))
	case m.page == pageProgress:
		m.viewSteps(&b)
	case m.page == pageSummary:
		m.viewSummary(&b)
	default:
		m.viewMenuPage(&b)
	}
	v := tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(b.String()))
	v.AltScreen = true
	return v
}

func (m *Model) wrap(s string) string { return m.wrapIndent(s, 0) }

// wrapIndent wraps s to the screen width, indenting every line.
func (m *Model) wrapIndent(s string, indent int) string {
	width := 76
	if m.width > 0 {
		width = min(width, m.width-4)
	}
	return lipgloss.NewStyle().Width(width).PaddingLeft(indent).Render(s)
}

func row(label, value string) string {
	return dim.Render(fmt.Sprintf("%-13s", label)) + value + "\n"
}

func yesNo(b bool) string {
	if b {
		return good.Render("yes")
	}
	return "no"
}

func (m *Model) viewMenuPage(b *strings.Builder) {
	st := m.status
	switch m.page {
	case pageHome:
		plasma := st.Plasma
		if plasma == "" {
			plasma = "not found"
		}
		b.WriteString(row("Plasma", plasma))
		b.WriteString(row("Theme", m.themeSummary()))
		b.WriteString(row("Components", m.packageSummary()))
		if st.Plasma != "" && !st.PlasmaSupported() {
			b.WriteString("\n" + m.wrap(warning.Render(fmt.Sprintf("Protea is made for Plasma %s. On Plasma %s some parts may look wrong.",
				system.TargetPlasma, system.Feature(st.Plasma)))) + "\n")
		}
	case pageTheme:
		b.WriteString(bold.Render("Theme") + "\n\n")
		b.WriteString(row("Installed", yesNo(st.CoreInstalled)))
		b.WriteString(row("In use", m.themeInUse()))
	case pageComponents:
		b.WriteString(bold.Render("Optional components") + "\n\n")
		m.viewComponentStatus(b)
	}

	b.WriteString("\n")
	items := m.items()
	for i, it := range items {
		cursor, label := "  ", it.label
		if it.toggle != nil {
			box := "[ ]"
			if *it.toggle {
				box = "[" + accent.Render("x") + "]"
			}
			label = box + " " + label
		}
		if i == m.cursor {
			cursor, label = accent.Render("› "), bold.Render(label)
		}
		b.WriteString(cursor + label + "\n")
	}
	if m.cursor < len(items) && items[m.cursor].hint != "" {
		b.WriteString("\n" + m.wrap(dim.Render(items[m.cursor].hint)) + "\n")
	}
	help := "↑/↓ move · enter select · q quit"
	if m.page != pageHome {
		help = "↑/↓ move · enter select · esc back · q quit"
	}
	b.WriteString("\n" + dim.Render(help))
}

func (m *Model) themeInUse() string {
	st := m.status
	var parts []string
	if st.GlobalThemeActive {
		parts = append(parts, "Global Theme")
	}
	if st.ColorsActive {
		parts = append(parts, "colors")
	}
	if st.PlasmaStyleActive {
		parts = append(parts, "Plasma Style")
	}
	if len(parts) == 0 {
		return "no"
	}
	return good.Render(strings.Join(parts, ", "))
}

func (m *Model) themeSummary() string {
	if !m.status.CoreInstalled {
		return "not installed"
	}
	if in := m.themeInUse(); in != "no" {
		return "installed · in use: " + in
	}
	return "installed · not in use"
}

func (m *Model) components() string {
	var parts []string
	if m.status.BuiltStyle() {
		parts = append(parts, "application style")
	}
	if m.status.BuiltDecoration() {
		parts = append(parts, "window decoration")
	}
	return strings.Join(parts, " + ")
}

func (m *Model) packageSummary() string {
	if m.status.Package == "" {
		return "not installed"
	}
	return "installed (" + m.components() + ")"
}

func (m *Model) viewComponentStatus(b *strings.Builder) {
	st := m.status
	if st.Package == "" {
		b.WriteString(row("Package", "not installed"))
	} else {
		b.WriteString(row("Package", "protea "+st.Package+" · "+m.components()))
		built := "Qt " + st.Build["QT_VERSION"]
		if st.BuiltDecoration() {
			built = "Plasma " + st.BuiltFor() + " · " + built
		}
		b.WriteString(row("Built for", built))
		b.WriteString(row("In use", "application style: "+yesNo(st.StyleActive)+" · window decoration: "+yesNo(st.DecorationActive)))
	}
	tools := good.Render("installed")
	if len(st.MissingTools) > 0 {
		tools = warning.Render("missing " + strings.Join(st.MissingTools, ", "))
	}
	b.WriteString(row("Build tools", tools))

	var notes []string
	switch {
	case st.Mismatch():
		notes = append(notes, bad.Render(fmt.Sprintf("The window decoration was built for Plasma %s, but Plasma %s is installed. Switch back, then build again.",
			st.BuiltFor(), system.Feature(st.Kdecoration))))
	case st.UpgradeBlocked():
		notes = append(notes, warning.Render(fmt.Sprintf("Plasma %s is available, but the package's version pin blocks the upgrade so KWin never loads a mismatched decoration. Choose \"Prepare for Plasma %[1]s\", run sudo pacman -Syu, then build again.",
			system.Feature(st.KdecorationUpgrade))))
	case st.Package != "" && st.BuiltDecoration():
		notes = append(notes, dim.Render(fmt.Sprintf("The window decoration is pinned to Plasma %s: pacman won't upgrade Plasma to the next feature release until the package is removed.", st.BuiltFor())))
	}
	for _, n := range notes {
		b.WriteString("\n" + m.wrap(n) + "\n")
	}
}

func (m *Model) viewSteps(b *strings.Builder) {
	b.WriteString(bold.Render(m.task) + "\n\n")
	for _, s := range m.steps {
		var mark string
		switch s.state {
		case stepPending:
			mark = dim.Render("·")
		case stepRunning:
			mark = m.spinner.View()
		case stepDone:
			mark = good.Render("✓")
		case stepFailed:
			mark = bad.Render("✗")
		case stepSkipped:
			mark = dim.Render("–")
		}
		name := s.name
		if s.state == stepSkipped || s.state == stepPending {
			name = dim.Render(name)
		}
		b.WriteString(mark + " " + name + "\n")
		if s.err != nil {
			b.WriteString(m.wrapIndent(bad.Render(s.err.Error()), 2) + "\n")
		}
		for _, l := range s.log {
			b.WriteString(m.wrapIndent(dim.Render(l), 2) + "\n")
		}
	}
}

func (m *Model) viewSummary(b *strings.Builder) {
	m.viewSteps(b)
	if m.reason != "" {
		b.WriteString(m.wrap(bad.Render(m.reason)) + "\n")
	}
	b.WriteString("\n")
	if m.failed {
		b.WriteString(bad.Render("Didn't finish.") + "\n")
	} else {
		b.WriteString(good.Render("Done.") + "\n")
		if m.note != "" {
			b.WriteString(m.wrap(m.note) + "\n")
		}
	}
	b.WriteString("\n" + dim.Render("enter back · q quit"))
}
