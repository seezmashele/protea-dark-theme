// Package tui is the installer's terminal interface: a step-by-step flow
// that collects choices page by page, shows the resulting plan for review,
// and only then makes changes.
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

	"proteadesktop.org/protea-theme/internal/system"
)

type page int

const (
	pageWelcome page = iota
	pageTheme
	pageComponents
	pageRemove
	pageReview
	pageRun
	pageDone
)

type mode int

const (
	modeInstall mode = iota
	modeUninstall
	modePrepare // remove the package so pacman can upgrade Plasma
)

type themeChoice int

const (
	themeGlobal themeChoice = iota
	themeColors
	themeFilesOnly
)

// plan holds the choices made on the way to the Review page.
type plan struct {
	mode  mode
	theme themeChoice

	style      bool // components to have installed
	decoration bool
	use        bool // switch to the components once installed
	rebuild    bool // build again even if the installed package is current

	removeTheme   bool
	removePackage bool
}

// row is one line of a page's choices.
type row struct {
	label, hint string
	check       *bool          // checkbox row
	radio       bool           // radio row: selected when the cursor is on it
	next        func() tea.Cmd // what enter does on this row, if not the page's default
}

type stepState int

const (
	stepPending stepState = iota
	stepRunning
	stepDone
	stepFailed
	stepSkipped
)

// step is one part of the plan. Quick steps run in the background; steps
// that need the terminal (sudo, makepkg) run through tea.ExecProcess.
type step struct {
	name   string
	run    func() error
	exec   func() *exec.Cmd
	always bool // run even after an earlier step failed (cleanup)
	quiet  bool // housekeeping, not listed on the Review page

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
	plan   plan

	steps   []step
	failed  bool
	spinner spinner.Model
}

// New returns the installer model. readStatus inspects the system; tests
// pass a fake.
func New(files fs.FS, runner *system.Runner, readStatus func() system.Status) *Model {
	return &Model{
		files:      files,
		runner:     runner,
		readStatus: readStatus,
		spinner:    spinner.New(spinner.WithSpinner(dot()), spinner.WithStyle(accent)),
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
		if m.page == pageWelcome {
			m.cursor = min(m.cursor, max(len(m.rows())-1, 0))
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case spinner.TickMsg:
		if m.page == pageRun {
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
	switch {
	case k == "ctrl+c":
		return tea.Quit
	case m.page == pageRun:
		return nil
	case k == "q":
		return tea.Quit
	case !m.loaded:
		return nil
	case m.page == pageDone:
		if k == "enter" {
			m.open(pageWelcome)
			return m.refresh()
		}
		return nil
	}

	rows := m.rows()
	if len(rows) == 0 {
		if k == "esc" || k == "backspace" {
			m.back()
		}
		return nil
	}
	switch k {
	case "up", "k":
		m.move(-1, len(rows))
	case "down", "j":
		m.move(1, len(rows))
	case "space":
		if r := rows[m.cursor]; r.check != nil {
			*r.check = !*r.check
		}
	case "esc", "backspace":
		m.back()
	case "enter":
		if r := rows[m.cursor]; r.next != nil {
			return r.next()
		}
		m.forward()
	}
	return nil
}

func (m *Model) move(by, count int) {
	m.cursor = min(max(m.cursor+by, 0), count-1)
	if m.page == pageTheme {
		m.plan.theme = themeChoice(m.cursor)
	}
}

func (m *Model) open(p page) {
	m.page, m.cursor = p, 0
	switch p {
	case pageWelcome:
		m.plan = plan{}
	case pageTheme:
		m.cursor = int(m.plan.theme)
	}
}

// forward goes to the next page of the current mode.
func (m *Model) forward() {
	switch m.page {
	case pageTheme:
		m.open(pageComponents)
	case pageComponents, pageRemove:
		m.open(pageReview)
	}
}

// back goes to the previous page of the current mode.
func (m *Model) back() {
	switch m.page {
	case pageTheme, pageRemove:
		m.open(pageWelcome)
	case pageComponents:
		m.open(pageTheme)
	case pageReview:
		switch m.plan.mode {
		case modeInstall:
			m.open(pageComponents)
		case modeUninstall:
			m.open(pageRemove)
		default:
			m.open(pageWelcome)
		}
	}
}

// startInstall begins the install flow, pre-filled from what's installed.
func (m *Model) startInstall() tea.Cmd {
	st := m.status
	p := plan{mode: modeInstall, theme: themeGlobal, use: true}
	if !st.GlobalThemeActive && (st.ColorsActive || st.PlasmaStyleActive) {
		p.theme = themeColors
	}
	if st.Package != "" {
		p.style, p.decoration = st.BuiltStyle(), st.BuiltDecoration()
		p.use = st.StyleActive || st.DecorationActive
	}
	m.plan = p
	m.open(pageTheme)
	return nil
}

func (m *Model) startUninstall() tea.Cmd {
	st := m.status
	m.plan = plan{mode: modeUninstall, removeTheme: m.themePresent(), removePackage: st.Package != ""}
	m.open(pageRemove)
	return nil
}

func (m *Model) startPrepare() tea.Cmd {
	m.plan = plan{mode: modePrepare}
	m.open(pageReview)
	return nil
}

func (m *Model) themePresent() bool {
	st := m.status
	return st.CoreInstalled || st.GlobalThemeActive || st.ColorsActive || st.PlasmaStyleActive
}

func (m *Model) somethingInstalled() bool { return m.themePresent() || m.status.Package != "" }

// rows are the choices on the current page.
func (m *Model) rows() []row {
	st := m.status
	switch m.page {
	case pageWelcome:
		install := row{label: "Install Protea Dark", next: m.startInstall,
			hint: "Choose how to use the theme and whether to build the optional components, then review before anything changes."}
		if m.somethingInstalled() {
			install.label = "Update or change Protea Dark"
		}
		rows := []row{install}
		if st.UpgradeBlocked() {
			rows = append(rows, row{label: "Prepare for Plasma " + system.Feature(st.KdecorationUpgrade), next: m.startPrepare,
				hint: "Switches back from Protea's components and removes the package, so pacman can upgrade Plasma."})
		}
		if m.somethingInstalled() {
			rows = append(rows, row{label: "Uninstall", next: m.startUninstall,
				hint: "Switches back to your previous look and removes what Protea installed."})
		}
		return rows
	case pageTheme:
		return []row{
			{label: "Full Global Theme", radio: true,
				hint: "Protea Dark's colors and Plasma Style as a Global Theme. Also switches icons, cursors, application style and title bars to Breeze's; uninstalling switches them back. Your panel layout stays as it is."},
			{label: "Colors and Plasma Style", radio: true,
				hint: "Only Protea Dark's colors and Plasma Style. Keeps your icons, cursors, application style and title bars."},
			{label: "Install only", radio: true,
				hint: "Copies the theme files without changing your current look. Choose Protea Dark later in System Settings."},
		}
	case pageComponents:
		rows := []row{
			{label: "Application style", check: &m.plan.style, hint: "Protea's look for buttons, menus, scrollbars and other controls in Qt apps."},
			{label: "Window decoration", check: &m.plan.decoration, hint: "Protea's title bars and window borders, drawn by KWin."},
			{label: "Use them once installed", check: &m.plan.use, hint: "Switches your application style and window decoration to the selected components."},
		}
		if st.Package != "" {
			rows = append(rows, row{label: "Build again", check: &m.plan.rebuild,
				hint: "Rebuilds even if the installed package is current, for example after changing the C++ code."})
		}
		return rows
	case pageRemove:
		var rows []row
		if m.themePresent() {
			rows = append(rows, row{label: "Protea Dark theme", check: &m.plan.removeTheme,
				hint: "Switches back to your previous colors, Plasma Style and Global Theme settings, then removes the theme files."})
		}
		if st.Package != "" {
			rows = append(rows, row{label: "Application style and window decoration", check: &m.plan.removePackage,
				hint: "Switches back to your previous application style and window decoration, then removes the protea package with pacman."})
		}
		return rows
	case pageReview:
		if len(m.reviewSteps()) == 0 {
			return []row{{label: "Back", next: func() tea.Cmd { m.back(); return nil }}}
		}
		return []row{
			{label: m.actionName(), next: m.run},
			{label: "Back", next: func() tea.Cmd { m.back(); return nil }},
		}
	}
	return nil
}

// flow is the page names shown in the progress line for the current mode.
func (m *Model) flow() []string {
	switch m.plan.mode {
	case modeUninstall:
		return []string{"Welcome", "Remove", "Review", "Uninstall"}
	case modePrepare:
		return []string{"Welcome", "Review", "Prepare"}
	}
	return []string{"Welcome", "Theme", "Components", "Review", "Install"}
}

func (m *Model) actionName() string { return m.flow()[len(m.flow())-1] }

// position is the current page's place in flow.
func (m *Model) position() int {
	last := len(m.flow()) - 1
	switch m.page {
	case pageWelcome:
		return 0
	case pageTheme, pageRemove:
		return 1
	case pageComponents:
		return 2
	case pageReview:
		return last - 1
	case pageRun:
		return last
	}
	return last + 1 // done: everything finished
}

// planSteps turns the plan into the steps that will run.
func (m *Model) planSteps() []step {
	switch m.plan.mode {
	case modeUninstall:
		return m.uninstallSteps()
	case modePrepare:
		var steps []step
		if m.status.StyleActive || m.status.DecorationActive {
			steps = append(steps, step{name: "Switch back to your previous application style and window decoration", run: m.runner.RevertComponents})
		}
		return append(steps, m.removePackageStep())
	}
	return m.installSteps()
}

// reviewSteps are the steps listed on the Review page.
func (m *Model) reviewSteps() []step {
	var steps []step
	for _, s := range m.planSteps() {
		if !s.quiet {
			steps = append(steps, s)
		}
	}
	return steps
}

// needsBuild reports whether the plan's components must be (re)built.
func (p plan) needsBuild(st system.Status) bool {
	if !p.style && !p.decoration {
		return false
	}
	return st.Package == "" || p.rebuild || st.Mismatch() ||
		p.style != st.BuiltStyle() || p.decoration != st.BuiltDecoration()
}

func componentNames(style, decoration bool) string {
	switch {
	case style && decoration:
		return "application style and window decoration"
	case style:
		return "application style"
	}
	return "window decoration"
}

func (m *Model) installSteps() []step {
	st, p := m.status, m.plan
	build := p.needsBuild(st)
	var steps []step

	if build && len(st.MissingTools) > 0 {
		steps = append(steps, step{name: "Install the build tools: " + strings.Join(st.MissingTools, ", "), exec: m.runner.ToolsCommand})
	}

	copyName := "Copy Protea Dark into " + tildePath(system.DataHome())
	if st.CoreInstalled {
		copyName = "Update Protea Dark in " + tildePath(system.DataHome())
	}
	steps = append(steps, step{name: copyName, run: func() error { return m.runner.InstallCore(m.files) }})

	// Switch back from components that are being dropped. Done before the
	// Global Theme, which sets its own style and decoration.
	keepStyle, keepDecoration := p.style && p.use, p.decoration && p.use
	if st.StyleActive && !keepStyle {
		steps = append(steps, step{name: "Switch back to your previous application style", run: m.runner.RevertStyle})
	}
	if st.DecorationActive && !keepDecoration {
		steps = append(steps, step{name: "Switch back to your previous window decoration", run: m.runner.RevertDecoration})
	}

	styleActive, decorationActive := st.StyleActive, st.DecorationActive
	switch {
	case p.theme == themeGlobal && !st.GlobalThemeActive:
		steps = append(steps, step{name: "Apply the Protea Dark Global Theme", run: m.runner.ApplyGlobalTheme})
		styleActive, decorationActive = false, false
	case p.theme == themeColors && !(st.ColorsActive && st.PlasmaStyleActive):
		steps = append(steps, step{name: "Switch colors and Plasma Style to Protea Dark", run: m.runner.ApplyColors})
	}

	switch {
	case build:
		var dir string
		steps = append(steps,
			step{name: "Copy the C++ sources to a temporary folder", quiet: true, run: func() (err error) {
				dir, err = m.runner.PrepareBuild(m.files)
				return err
			}},
			step{name: "Build and install the " + componentNames(p.style, p.decoration) + " with makepkg", exec: func() *exec.Cmd {
				return m.runner.BuildCommand(dir, p.style, p.decoration)
			}},
			step{name: "Remove the temporary build folder", quiet: true, always: true, run: func() error {
				if dir == "" {
					return nil
				}
				return m.runner.CleanBuild(dir)
			}},
		)
	case !p.style && !p.decoration && st.Package != "":
		steps = append(steps, m.removePackageStep())
	}

	if keepStyle && !styleActive {
		steps = append(steps, step{name: "Switch the application style to Protea", run: m.runner.UseStyle})
	}
	if keepDecoration && !decorationActive {
		steps = append(steps, step{name: "Switch the window decoration to Protea", run: m.runner.UseDecoration})
	}
	return steps
}

func (m *Model) uninstallSteps() []step {
	st, p := m.status, m.plan
	var steps []step
	if p.removePackage && st.Package != "" {
		if st.StyleActive || st.DecorationActive {
			steps = append(steps, step{name: "Switch back to your previous application style and window decoration", run: m.runner.RevertComponents})
		}
		steps = append(steps, m.removePackageStep())
	}
	if p.removeTheme && m.themePresent() {
		steps = append(steps, step{name: "Switch back to your previous theme settings", run: m.runner.RevertCore})
		if st.CoreInstalled {
			steps = append(steps, step{name: "Remove the theme files", run: m.runner.UninstallCore})
		}
	}
	return steps
}

func (m *Model) removePackageStep() step {
	return step{name: "Remove the protea package with pacman", exec: func() *exec.Cmd { return m.runner.RemoveCommand(true) }}
}

// run starts the plan's steps.
func (m *Model) run() tea.Cmd {
	m.steps, m.failed = m.planSteps(), false
	m.page = pageRun
	return tea.Batch(m.spinner.Tick, m.runStep(0))
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
	m.page = pageDone
	return m.refresh()
}

// tildePath shortens a path in the home folder to ~/...
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+"/") {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func (m *Model) View() tea.View {
	var b strings.Builder
	b.WriteString(title.Render("Protea Dark") + "  " + m.progressLine())
	if m.runner.DryRun {
		b.WriteString("\n" + warning.Render("Dry run: nothing will be changed"))
	}
	b.WriteString("\n\n")
	switch {
	case !m.loaded:
		b.WriteString(dim.Render("Checking your system…"))
	case m.page == pageWelcome:
		m.viewWelcome(&b)
	case m.page == pageTheme:
		b.WriteString(m.wrap("The theme files are always installed into "+tildePath(system.DataHome())+". How should Protea Dark be used?") + "\n\n")
		m.viewRows(&b, "↑/↓ choose · enter next · esc back · q quit")
	case m.page == pageComponents:
		m.viewComponents(&b)
	case m.page == pageRemove:
		b.WriteString("What should be removed?\n\n")
		m.viewRows(&b, "↑/↓ move · space tick · enter next · esc back · q quit")
	case m.page == pageReview:
		m.viewReview(&b)
	case m.page == pageRun:
		m.viewSteps(&b)
	case m.page == pageDone:
		m.viewDone(&b)
	}
	v := tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(b.String()))
	v.AltScreen = true
	return v
}

func (m *Model) progressLine() string {
	pos := m.position()
	var parts []string
	for i, name := range m.flow() {
		switch {
		case i < pos:
			parts = append(parts, good.Render("✓")+" "+dim.Render(name))
		case i == pos:
			parts = append(parts, accent.Render("●")+" "+bold.Render(name))
		default:
			parts = append(parts, dim.Render("○ "+name))
		}
	}
	return strings.Join(parts, dim.Render(" ─ "))
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

func field(label, value string) string {
	return dim.Render(fmt.Sprintf("%-13s", label)) + value + "\n"
}

// viewRows draws the page's rows, the selected row's hint, any notes, and
// the key help.
func (m *Model) viewRows(b *strings.Builder, help string, notes ...string) {
	rows := m.rows()
	for i, r := range rows {
		label := r.label
		switch {
		case r.check != nil:
			box := "[ ]"
			if *r.check {
				box = "[" + accent.Render("x") + "]"
			}
			label = box + " " + label
		case r.radio:
			dotMark := "( )"
			if i == m.cursor {
				dotMark = "(" + accent.Render("•") + ")"
			}
			label = dotMark + " " + label
		}
		cursor := "  "
		if i == m.cursor {
			cursor, label = accent.Render("› "), bold.Render(label)
		}
		b.WriteString(cursor + label + "\n")
	}
	if m.cursor < len(rows) && rows[m.cursor].hint != "" {
		b.WriteString("\n" + m.wrap(dim.Render(rows[m.cursor].hint)) + "\n")
	}
	for _, n := range notes {
		b.WriteString("\n" + m.wrap(n) + "\n")
	}
	b.WriteString("\n" + dim.Render(help))
}

func (m *Model) viewWelcome(b *strings.Builder) {
	st := m.status
	plasma := st.Plasma
	switch {
	case plasma == "":
		plasma = warning.Render("not found")
	case st.PlasmaSupported():
		plasma += " " + good.Render("✓")
	}
	b.WriteString(bold.Render("Your system") + "\n")
	b.WriteString(field("Plasma", plasma))
	b.WriteString(field("Protea Dark", m.themeSummary()))
	b.WriteString(field("Components", m.packageSummary()))
	if st.Plasma != "" && !st.PlasmaSupported() {
		b.WriteString("\n" + m.wrap(warning.Render(fmt.Sprintf("Protea is made for Plasma %s. On Plasma %s some parts may look wrong.",
			system.TargetPlasma, system.Feature(st.Plasma)))) + "\n")
	}
	if st.UpgradeBlocked() {
		b.WriteString("\n" + m.wrap(warning.Render(fmt.Sprintf("Plasma %s is available, but the protea package's version pin blocks the upgrade so KWin never loads a mismatched window decoration.",
			system.Feature(st.KdecorationUpgrade)))) + "\n")
	}
	if st.Mismatch() {
		b.WriteString("\n" + m.wrap(bad.Render(fmt.Sprintf("The window decoration was built for Plasma %s, but Plasma %s is installed. Update to build it again.",
			st.BuiltFor(), system.Feature(st.Kdecoration)))) + "\n")
	}
	b.WriteString("\n" + bold.Render("What would you like to do?") + "\n")
	m.viewRows(b, "↑/↓ choose · enter next · q quit")
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
	return strings.Join(parts, ", ")
}

func (m *Model) themeSummary() string {
	in := m.themeInUse()
	switch {
	case !m.status.CoreInstalled:
		return "not installed"
	case in != "":
		return "installed · in use: " + good.Render(in)
	}
	return "installed · not in use"
}

func (m *Model) packageSummary() string {
	st := m.status
	if st.Package == "" {
		return "not installed"
	}
	var used []string
	if st.StyleActive {
		used = append(used, "application style")
	}
	if st.DecorationActive {
		used = append(used, "window decoration")
	}
	s := componentNames(st.BuiltStyle(), st.BuiltDecoration())
	if len(used) > 0 {
		return s + " · in use: " + good.Render(strings.Join(used, ", "))
	}
	return s + " · not in use"
}

func (m *Model) viewComponents(b *strings.Builder) {
	st := m.status
	b.WriteString(m.wrap("Optional. Compiled from source with makepkg, which takes about a minute and asks for your sudo password.") + "\n\n")

	var notes []string
	if st.Package != "" {
		built := "Installed: protea " + st.Package + ", Qt " + st.Build["QT_VERSION"]
		if st.BuiltDecoration() {
			built += ", Plasma " + st.BuiltFor()
		}
		notes = append(notes, dim.Render(built+"."))
	}
	if m.plan.needsBuild(st) && len(st.MissingTools) > 0 {
		notes = append(notes, warning.Render("Missing build tools ("+strings.Join(st.MissingTools, ", ")+") will be installed first."))
	}
	if m.plan.decoration {
		notes = append(notes, dim.Render(fmt.Sprintf("The window decoration is pinned to Plasma %s: pacman won't upgrade Plasma to the next feature release until the package is removed. This installer walks you through that.",
			system.Feature(st.Kdecoration))))
	}
	m.viewRows(b, "↑/↓ move · space tick · enter next · esc back · q quit", notes...)
}

func (m *Model) viewReview(b *strings.Builder) {
	steps := m.reviewSteps()
	if len(steps) == 0 {
		b.WriteString("Nothing to change.\n\n")
		m.viewRows(b, "enter back · q quit")
		return
	}
	b.WriteString(bold.Render("This will:") + "\n")
	sudo := false
	for i, s := range steps {
		line := fmt.Sprintf("  %d  %s", i+1, s.name)
		if s.exec != nil {
			line += "  " + warning.Render("needs sudo")
			sudo = true
		}
		b.WriteString(m.wrap(line) + "\n")
	}
	p := m.plan
	switch p.mode {
	case modeInstall:
		if p.theme != themeFilesOnly || (p.use && (p.style || p.decoration)) {
			b.WriteString("\n" + m.wrap(dim.Render("Your current settings are saved first, so Uninstall can put them back.")) + "\n")
		}
	case modePrepare:
		b.WriteString("\n" + m.wrap(dim.Render("Afterwards, run sudo pacman -Syu, then run this installer again to build for the new Plasma.")) + "\n")
	}
	if sudo {
		b.WriteString(m.wrap(dim.Render("Steps needing sudo take over the terminal; press Enter afterwards to come back.")) + "\n")
	}
	b.WriteString("\n")
	m.viewRows(b, "↑/↓ choose · enter select · esc back · q quit")
}

func (m *Model) viewSteps(b *strings.Builder) {
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

func (m *Model) viewDone(b *strings.Builder) {
	m.viewSteps(b)
	b.WriteString("\n")
	switch {
	case m.failed:
		b.WriteString(bad.Render("Didn't finish.") + " " + dim.Render("Nothing after the failed step was done, except cleanup.") + "\n")
	case m.plan.mode == modePrepare:
		b.WriteString(good.Render("Done.") + "\n" + m.wrap("Now run sudo pacman -Syu, then run this installer again to build for the new Plasma.") + "\n")
	default:
		b.WriteString(good.Render("Done.") + "\n")
		if m.ranStep("Switch the application style to Protea") {
			b.WriteString(m.wrap(dim.Render("Some apps need a restart to show the new application style.")) + "\n")
		}
	}
	b.WriteString("\n" + dim.Render("enter start over · q quit"))
}

func (m *Model) ranStep(name string) bool {
	for _, s := range m.steps {
		if s.name == name && s.state == stepDone {
			return true
		}
	}
	return false
}
