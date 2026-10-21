// Package tui is the installer's terminal interface: a step-by-step flow
// that collects choices page by page, shows the resulting plan for review,
// and only then makes changes. It installs Protea; applying it is left to
// the user, in System Settings.
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
	pageColors
	pageOptions
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

// plan holds the choices made on the way to the Review page.
type plan struct {
	mode mode

	accent     string // accent color for the theme, #rrggbb
	tasks      string // how the task manager marks open apps, system.TasksUnderline or TasksBackground
	components bool   // have the C++ components installed
	rebuild    bool   // build again even if the installed package is current
	buttons    string // the window decoration's button shape, system.ButtonsRound or ButtonsRectangle

	removeTheme   bool
	removePackage bool
}

// row is one line of a page's choices.
type row struct {
	label, hint string
	detail      string  // shown faded after the label
	swatch      string  // a color shown before the label, #rrggbb
	radio       bool    // radio row: selected when the cursor is on it
	check       *bool   // checkbox row
	choice      *string // option row: selected when *choice is value; space picks it
	value       string
	disabled    string         // why the row can't be chosen; grayed out and skipped
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

	steps    []step
	failed   bool
	reapply  bool // the run changed the accent of a theme that's in use
	restyled bool // the run changed the task manager style of a Plasma Style that's in use
	rebuilt  bool // the run built the C++ components
	spinner  spinner.Model
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

func (m *Model) Init() tea.Cmd { return m.refresh() }

func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg { return statusMsg(m.readStatus()) }
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.status, m.loaded = system.Status(msg), true
		if m.page == pageWelcome {
			m.cursor = 0
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
		m.move(rows, -1)
	case "down", "j":
		m.move(rows, 1)
	case "space":
		if r := rows[m.cursor]; r.check != nil {
			*r.check = !*r.check
		} else if r.choice != nil {
			*r.choice = r.value
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

// move steps the cursor by one row, skipping disabled rows.
func (m *Model) move(rows []row, by int) {
	for i := m.cursor + by; i >= 0 && i < len(rows); i += by {
		if rows[i].disabled == "" {
			m.cursor = i
			break
		}
	}
	if m.page == pageColors {
		m.plan.accent = system.Accents[m.cursor].Hex
	}
}

func (m *Model) open(p page) {
	m.page, m.cursor = p, 0
	switch p {
	case pageWelcome:
		m.plan = plan{}
	case pageColors:
		for i, a := range system.Accents {
			if a.Hex == m.plan.accent {
				m.cursor = i
			}
		}
	}
}

// forward goes to the next page of the current mode.
func (m *Model) forward() {
	switch m.page {
	case pageColors:
		m.open(pageOptions)
	case pageOptions, pageRemove:
		m.open(pageReview)
	}
}

// back goes to the previous page of the current mode.
func (m *Model) back() {
	switch m.page {
	case pageColors, pageRemove:
		m.open(pageWelcome)
	case pageOptions:
		m.open(pageColors)
	case pageReview:
		switch m.plan.mode {
		case modeInstall:
			m.open(pageOptions)
		case modeUninstall:
			m.open(pageRemove)
		default:
			m.open(pageWelcome)
		}
	}
}

func (m *Model) startInstall() tea.Cmd {
	accent := system.DefaultAccent
	if installed := m.status.Accent; system.AccentName(installed) != installed {
		accent = installed // a preset: keep it
	}
	m.plan = plan{mode: modeInstall, accent: accent, tasks: taskStyle(m.status), components: m.status.Package != "", buttons: buttonShape(m.status)}
	m.open(pageColors)
	return nil
}

func (m *Model) startUninstall() tea.Cmd {
	m.plan = plan{mode: modeUninstall, removeTheme: m.themePresent(), removePackage: m.status.Package != ""}
	m.open(pageRemove)
	return nil
}

func (m *Model) startPrepare() tea.Cmd {
	m.plan = plan{mode: modePrepare}
	m.open(pageReview)
	return nil
}

// buttonShape is the window decoration's button shape, rectangular (its
// default) if it isn't set.
func buttonShape(st system.Status) string {
	if st.ButtonShape == "" {
		return system.ButtonsRectangle
	}
	return st.ButtonShape
}

// taskStyle is the installed task manager style, underline (the theme's
// own) if there's none or it's unknown.
func taskStyle(st system.Status) string {
	if st.TaskStyle == system.TasksBackground {
		return system.TasksBackground
	}
	return system.TasksUnderline
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
			hint: "Installs the theme and, if you choose, the C++ components. You apply them yourself afterwards in System Settings."}
		if m.somethingInstalled() {
			install.label = "Update Protea Dark"
			install.hint = "Updates the installed theme files, and adds, rebuilds or removes the C++ components."
		}
		rows := []row{install}
		if st.UpgradeBlocked() {
			rows = append(rows, row{label: "Prepare for Plasma " + system.Feature(st.KdecorationUpgrade), next: m.startPrepare,
				hint: "Switches back from Protea's C++ components and removes them, so pacman can upgrade Plasma."})
		}
		uninstall := row{label: "Uninstall", next: m.startUninstall,
			hint: "Switches back to Breeze Dark where Protea is in use, then removes what Protea installed."}
		if !m.somethingInstalled() {
			uninstall.disabled = "nothing installed"
		}
		return append(rows, uninstall)
	case pageColors:
		var rows []row
		for _, a := range system.Accents {
			rows = append(rows, row{label: a.Name, detail: a.Hex, swatch: a.Hex, radio: true})
		}
		return rows
	case pageOptions:
		rows := []row{
			{label: "Task manager: underline", detail: "a line under open apps", choice: &m.plan.tasks, value: system.TasksUnderline,
				hint: "Each open app in the panel gets a line along its bottom edge, and the focused app is tinted with the accent. Pointing at an app tints it."},
			{label: "Task manager: background", detail: "a tint behind open apps", choice: &m.plan.tasks, value: system.TasksBackground,
				hint: "Each open app in the panel gets a see-through grey box behind it, and the focused app's is tinted with the accent. Pointing at an app adds a line along its bottom edge."},
			{label: "C++ components", check: &m.plan.components,
				hint: "Protea's application style and window decoration, compiled from source with makepkg. Takes about a minute and asks for your sudo password."},
		}
		off := ""
		if !m.plan.components {
			off = "needs the C++ components"
		}
		rows = append(rows,
			row{label: "Rectangular window buttons", detail: "like Windows", choice: &m.plan.buttons, value: system.ButtonsRectangle, disabled: off,
				hint: "Wide title bar buttons side by side, flush with the top of the title bar, with Close in the corner."},
			row{label: "Round window buttons", detail: "like Breeze", choice: &m.plan.buttons, value: system.ButtonsRound, disabled: off,
				hint: "Title bar buttons with a circle behind the symbol when you point at them."})
		if st.Package != "" {
			rows = append(rows, row{label: "Build again", check: &m.plan.rebuild,
				hint: "Rebuilds even if the installed C++ components are current, for example after changing the C++ code."})
		}
		return rows
	case pageRemove:
		var rows []row
		if m.themePresent() {
			rows = append(rows, row{label: "Protea Dark theme", check: &m.plan.removeTheme,
				hint: "Switches to Breeze Dark if Protea Dark is in use, then removes the theme files."})
		}
		if st.Package != "" {
			rows = append(rows, row{label: "C++ components", check: &m.plan.removePackage,
				hint: "Switches the application style and window decoration to Breeze if Protea's are in use, then removes the protea package with pacman."})
		}
		return rows
	case pageReview:
		back := row{label: "Back", next: func() tea.Cmd { m.back(); return nil }}
		if len(m.reviewSteps()) == 0 {
			return []row{back}
		}
		return []row{{label: m.actionName(), next: m.run}, back}
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
	return []string{"Welcome", "Colors", "Options", "Review", "Install"}
}

func (m *Model) actionName() string { return m.flow()[len(m.flow())-1] }

// position is the current page's place in flow.
func (m *Model) position() int {
	last := len(m.flow()) - 1
	switch m.page {
	case pageWelcome:
		return 0
	case pageColors, pageRemove:
		return 1
	case pageOptions:
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
		return m.removePackageSteps()
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

// needsBuild reports whether the C++ components must be (re)built.
func (p plan) needsBuild(st system.Status) bool {
	if !p.components {
		return false
	}
	return st.Package == "" || p.rebuild || st.Mismatch() || !st.BuiltStyle() || !st.BuiltDecoration()
}

func (m *Model) installSteps() []step {
	st, p := m.status, m.plan
	build := p.needsBuild(st)
	var steps []step

	if build && len(st.MissingTools) > 0 {
		steps = append(steps, step{name: "Install the build tools: " + strings.Join(st.MissingTools, ", "), exec: m.runner.ToolsCommand})
	}

	copyName := "Install Protea Dark into " + tildePath(system.DataHome())
	if st.CoreInstalled {
		copyName = "Update Protea Dark in " + tildePath(system.DataHome())
	}
	copyName += ", with the " + system.AccentName(p.accent) + " accent and the " + p.tasks + " task manager"
	steps = append(steps, step{name: copyName, run: func() error { return m.runner.InstallCore(m.files, p.accent, p.tasks) }})

	switch {
	case build:
		var dir string
		steps = append(steps,
			step{name: "Copy the C++ sources to a temporary folder", quiet: true, run: func() (err error) {
				dir, err = m.runner.PrepareBuild(m.files)
				return err
			}},
			step{name: "Build and install the C++ components with makepkg", exec: func() *exec.Cmd {
				return m.runner.BuildCommand(dir)
			}},
			step{name: "Remove the temporary build folder", quiet: true, always: true, run: func() error {
				if dir == "" {
					return nil
				}
				return m.runner.CleanBuild(dir)
			}},
		)
	case !p.components && st.Package != "":
		steps = append(steps, m.removePackageSteps()...)
	}
	if p.components && p.buttons != buttonShape(st) {
		shape := map[string]string{system.ButtonsRound: "round", system.ButtonsRectangle: "rectangular"}[p.buttons]
		steps = append(steps, step{name: "Set the window buttons to " + shape, run: func() error { return m.runner.SetButtonShape(p.buttons) }})
	}
	return steps
}

func (m *Model) uninstallSteps() []step {
	st, p := m.status, m.plan
	var steps []step
	if p.removePackage && st.Package != "" {
		steps = append(steps, m.removePackageSteps()...)
	}
	if p.removeTheme && m.themePresent() {
		switch {
		case st.GlobalThemeActive:
			steps = append(steps, step{name: "Switch to the Breeze Dark Global Theme", run: m.runner.RevertCore})
		case st.ColorsActive || st.PlasmaStyleActive:
			steps = append(steps, step{name: "Switch colors and Plasma Style back to Breeze Dark's", run: m.runner.RevertCore})
		}
		if st.CoreInstalled {
			steps = append(steps, step{name: "Remove the theme files", run: m.runner.UninstallCore})
		}
	}
	return steps
}

// removePackageSteps switch away from the C++ components, if they're in
// use, and remove the package.
func (m *Model) removePackageSteps() []step {
	var steps []step
	if m.status.StyleActive || m.status.DecorationActive {
		steps = append(steps, step{name: "Switch the application style and window decoration back to Breeze", run: m.runner.RevertComponents})
	}
	return append(steps, step{name: "Remove the C++ components with pacman", exec: func() *exec.Cmd { return m.runner.RemoveCommand(true) }})
}

// accentNeedsReapply reports whether the plan changes the accent of a
// Protea Dark that's in use. Plasma copies a color scheme's colors when
// it's applied, so the new accent only shows after applying it again.
func (m *Model) accentNeedsReapply() bool {
	st := m.status
	return m.plan.mode == modeInstall && st.CoreInstalled && (st.GlobalThemeActive || st.ColorsActive) &&
		m.plan.accent != st.Accent
}

// run starts the plan's steps.
func (m *Model) run() tea.Cmd {
	m.reapply = m.accentNeedsReapply()
	m.restyled = m.plan.mode == modeInstall && m.status.CoreInstalled && m.status.PlasmaStyleActive && m.plan.tasks != m.status.TaskStyle
	m.rebuilt = m.plan.mode == modeInstall && m.plan.needsBuild(m.status)
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
	b.WriteString(logo + "\n\n" + m.progressLine() + "\n")
	if m.runner.DryRun {
		b.WriteString(warning.Render("Dry run: nothing will be changed") + "\n")
	}
	b.WriteString("\n")
	if !m.loaded {
		b.WriteString(dim.Render("Checking your system…"))
	} else {
		heading, description := m.heading()
		b.WriteString(marker.Render("▍") + pageTitle.Render(heading) + "\n")
		b.WriteString(m.wrap(description, text) + "\n\n")
		switch m.page {
		case pageWelcome:
			m.viewWelcome(&b)
		case pageColors:
			m.viewColors(&b)
		case pageOptions:
			m.viewOptions(&b)
		case pageRemove:
			m.viewRows(&b, "↑/↓ move · space tick · enter next · esc back · q quit")
		case pageReview:
			m.viewReview(&b)
		case pageRun:
			m.viewSteps(&b)
		case pageDone:
			m.viewDone(&b)
		}
	}
	v := tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(b.String()))
	v.AltScreen = true
	return v
}

// heading is the current page's title and the description under it.
func (m *Model) heading() (string, string) {
	action := m.actionName()
	switch m.page {
	case pageWelcome:
		return "Welcome", "Protea Dark is a dark theme for KDE Plasma, forked from Breeze. Here's what's on your system."
	case pageColors:
		return "Choose an accent color", "Protea Dark uses it for selections, focus rings, hover highlights and links, in apps, panels and the C++ components."
	case pageOptions:
		return "Choose what to install", "Protea Dark's Global Theme, Plasma Style and color scheme are always installed into " +
			tildePath(system.DataHome()) + ". The C++ components are optional."
	case pageRemove:
		return "Choose what to remove", "Anything of Protea's that's in use is switched to Breeze Dark first, so Plasma never points at missing files."
	case pageReview:
		return "Review the plan", "Nothing has changed yet. These steps run when you choose " + action + "."
	case pageRun:
		titles := map[mode]string{modeInstall: "Installing", modeUninstall: "Uninstalling", modePrepare: "Preparing for the Plasma upgrade"}
		return titles[m.plan.mode], "Steps that need sudo take over the terminal. Press Enter afterwards to come back here."
	}
	switch {
	case m.failed:
		return "Didn't finish", "A step failed. Nothing after it was done, except cleanup."
	case m.plan.mode == modeInstall:
		return "Installed", "Protea is ready. Apply it yourself in System Settings → Colors & Themes:"
	case m.plan.mode == modePrepare:
		return "Ready for the Plasma upgrade", "Now run sudo pacman -Syu, then run this installer again to build for the new Plasma."
	}
	return "Uninstalled", "Protea is removed."
}

func (m *Model) progressLine() string {
	pos := m.position()
	var parts []string
	for i, name := range m.flow() {
		switch {
		case i < pos:
			parts = append(parts, good.Render("✓ ")+dim.Render(name))
		case i == pos:
			parts = append(parts, accent.Render("● ")+strong.Render(name))
		default:
			parts = append(parts, dim.Render("○ "+name))
		}
	}
	return strings.Join(parts, dim.Render(" ─ "))
}

// wrap renders s in style st, wrapped to the screen width.
func (m *Model) wrap(s string, st lipgloss.Style) string { return m.wrapIndent(s, st, 0) }

// wrapIndent renders s in style st, wrapped to the screen width with every
// line indented.
func (m *Model) wrapIndent(s string, st lipgloss.Style, indent int) string {
	width := 76
	if m.width > 0 {
		width = min(width, m.width-4)
	}
	return st.Width(width).PaddingLeft(indent).Render(s)
}

func field(label, value string) string {
	return dim.Render(fmt.Sprintf("%-13s", label)) + text.Render(value) + "\n"
}

// viewRows draws the page's rows, the selected row's hint, any notes, and
// the key help.
func (m *Model) viewRows(b *strings.Builder, help string, notes ...string) {
	rows := m.rows()
	for i, r := range rows {
		box := ""
		if r.check != nil {
			box = text.Render("[ ] ")
			if *r.check {
				box = text.Render("[") + accent.Render("x") + text.Render("] ")
			}
		}
		if r.radio || r.choice != nil {
			box = text.Render("( ) ")
			if (r.radio && i == m.cursor) || (r.choice != nil && *r.choice == r.value) {
				box = text.Render("(") + accent.Render("•") + text.Render(") ")
			}
		}
		if r.swatch != "" {
			box += lipgloss.NewStyle().Foreground(lipgloss.Color(r.swatch)).Render("██") + " "
		}
		var label string
		switch {
		case r.disabled != "":
			label = dim.Render(r.label + " · " + r.disabled)
		case i == m.cursor:
			label = selected.Render(r.label)
		default:
			label = text.Render(r.label)
		}
		if r.detail != "" {
			label += "  " + dim.Render(r.detail)
		}
		cursor := "  "
		if i == m.cursor {
			cursor = accent.Render("› ")
		}
		b.WriteString(cursor + box + label + "\n")
	}
	if m.cursor < len(rows) && rows[m.cursor].hint != "" {
		b.WriteString("\n" + m.wrap(rows[m.cursor].hint, dim) + "\n")
	}
	for _, n := range notes {
		b.WriteString("\n" + n + "\n")
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
	b.WriteString(field("Plasma", plasma))
	b.WriteString(field("Protea Dark", m.themeSummary()))
	b.WriteString(field("C++", m.packageSummary()))
	if st.Plasma != "" && !st.PlasmaSupported() {
		b.WriteString("\n" + m.wrap(fmt.Sprintf("Protea is made for Plasma %s. On Plasma %s some parts may look wrong.",
			system.TargetPlasma, system.Feature(st.Plasma)), warning) + "\n")
	}
	if st.UpgradeBlocked() {
		b.WriteString("\n" + m.wrap(fmt.Sprintf("Plasma %s is available, but the protea package's version pin blocks the upgrade so KWin never loads a mismatched window decoration.",
			system.Feature(st.KdecorationUpgrade)), warning) + "\n")
	}
	if st.Mismatch() {
		b.WriteString("\n" + m.wrap(fmt.Sprintf("The C++ components were built for Plasma %s, but Plasma %s is installed. Update to build them again.",
			st.BuiltFor(), system.Feature(st.Kdecoration)), bad) + "\n")
	}
	b.WriteString("\n" + strong.Render("What would you like to do?") + "\n")
	m.viewRows(b, "↑/↓ choose · enter next · q quit")
}

func (m *Model) themeSummary() string {
	st := m.status
	var in []string
	if st.GlobalThemeActive {
		in = append(in, "Global Theme")
	}
	if st.ColorsActive {
		in = append(in, "colors")
	}
	if st.PlasmaStyleActive {
		in = append(in, "Plasma Style")
	}
	switch {
	case !st.CoreInstalled:
		return "not installed"
	case len(in) > 0:
		return "installed · in use: " + good.Render(strings.Join(in, ", "))
	}
	return "installed · not in use"
}

func (m *Model) packageSummary() string {
	st := m.status
	if st.Package == "" {
		return "not installed"
	}
	var in []string
	if st.StyleActive {
		in = append(in, "application style")
	}
	if st.DecorationActive {
		in = append(in, "window decoration")
	}
	if len(in) > 0 {
		return "installed · in use: " + good.Render(strings.Join(in, ", "))
	}
	return "installed · not in use"
}

func (m *Model) viewColors(b *strings.Builder) {
	var notes []string
	if selection, selectionText, link, err := system.AccentPreview(m.plan.accent); err == nil {
		sample := lipgloss.NewStyle().Background(lipgloss.Color(selection)).Foreground(lipgloss.Color(selectionText)).Render(" Selected item ")
		linkSample := lipgloss.NewStyle().Foreground(lipgloss.Color(link)).Underline(true).Render("a link")
		notes = append(notes, dim.Render("Preview  ")+sample+"  "+linkSample)
	}
	if m.accentNeedsReapply() {
		notes = append(notes, m.wrap("Protea Dark is in use: after installing, apply it again in System Settings to see the new accent.", warning))
	}
	m.viewRows(b, "↑/↓ choose · enter next · esc back · q quit", notes...)
}

func (m *Model) viewOptions(b *strings.Builder) {
	st := m.status
	var notes []string
	if st.Package != "" {
		built := "Installed: protea " + st.Package + ", built for Qt " + st.Build["QT_VERSION"]
		if st.BuiltDecoration() {
			built += " and Plasma " + st.BuiltFor()
		}
		notes = append(notes, m.wrap(built+".", dim))
	}
	if m.plan.needsBuild(st) && len(st.MissingTools) > 0 {
		notes = append(notes, m.wrap("Missing build tools ("+strings.Join(st.MissingTools, ", ")+") will be installed first.", warning))
	}
	if m.plan.components {
		notes = append(notes, m.wrap(fmt.Sprintf("The window decoration is pinned to Plasma %s: pacman won't upgrade Plasma to the next feature release until the C++ components are removed. This installer walks you through that.",
			system.Feature(st.Kdecoration)), dim))
	}
	m.viewRows(b, "↑/↓ move · space tick or pick · enter next · esc back · q quit", notes...)
}

func (m *Model) viewReview(b *strings.Builder) {
	steps := m.reviewSteps()
	if len(steps) == 0 {
		b.WriteString(text.Render("Nothing to change.") + "\n\n")
		m.viewRows(b, "enter back · q quit")
		return
	}
	for i, s := range steps {
		line := accent.Render(fmt.Sprintf("  %d  ", i+1)) + text.Render(s.name)
		if s.exec != nil {
			line += "  " + warning.Render("needs sudo")
		}
		b.WriteString(line + "\n")
	}
	switch m.plan.mode {
	case modeInstall:
		note := "Your current look doesn't change. Apply Protea Dark yourself in System Settings afterwards."
		if m.accentNeedsReapply() {
			note = "Protea Dark is in use: apply it again in System Settings afterwards to see the new accent."
		}
		b.WriteString("\n" + m.wrap(note, dim) + "\n")
	case modePrepare:
		b.WriteString("\n" + m.wrap("Afterwards, run sudo pacman -Syu, then run this installer again to build for the new Plasma.", dim) + "\n")
	}
	b.WriteString("\n")
	m.viewRows(b, "↑/↓ choose · enter select · esc back · q quit")
}

func (m *Model) viewSteps(b *strings.Builder) {
	for _, s := range m.steps {
		var mark, name string
		switch s.state {
		case stepPending:
			mark, name = dim.Render("·"), dim.Render(s.name)
		case stepRunning:
			mark, name = m.spinner.View(), strong.Render(s.name)
		case stepDone:
			mark, name = good.Render("✓"), text.Render(s.name)
		case stepFailed:
			mark, name = bad.Render("✗"), text.Render(s.name)
		case stepSkipped:
			mark, name = dim.Render("–"), dim.Render(s.name)
		}
		b.WriteString(mark + " " + name + "\n")
		if s.err != nil {
			b.WriteString(m.wrapIndent(s.err.Error(), bad, 2) + "\n")
		}
		for _, l := range s.log {
			b.WriteString(m.wrapIndent(l, dim, 2) + "\n")
		}
	}
}

func (m *Model) viewDone(b *strings.Builder) {
	if !m.failed && m.plan.mode == modeInstall {
		setting := func(page, choice string) string {
			return "  " + strong.Render(fmt.Sprintf("%-19s", page)) + accent.Render("→ ") + text.Render(choice) + "\n"
		}
		b.WriteString(setting("Global Theme", "Protea Dark"))
		b.WriteString(dim.Render("  or, to keep your icons and title bars, only:") + "\n")
		b.WriteString(setting("Colors", "Protea Dark"))
		b.WriteString(setting("Plasma Style", "Protea Dark"))
		if m.status.Package != "" {
			b.WriteString("\n" + m.wrap("Then pick Protea's window decoration and application style. The Global Theme sets Breeze's, so do this after applying it, and again whenever you reapply it:", warning) + "\n")
			b.WriteString(setting("Window Decorations", "Protea"))
			b.WriteString(setting("Application Style", "Protea"))
		}
		if m.reapply {
			b.WriteString("\n" + m.wrap("Protea Dark is already in use: apply it again to see the new accent.", warning) + "\n")
		}
		if m.restyled {
			b.WriteString("\n" + m.wrap("The task manager may keep its old look until you log out and back in.", warning) + "\n")
		}
		if m.rebuilt {
			b.WriteString("\n" + m.wrap("Log out and back in to load the new window decoration. KWin keeps the one it started with until then.", warning) + "\n")
		}
		b.WriteString("\n")
	}
	m.viewSteps(b)
	b.WriteString("\n" + dim.Render("enter start over · q quit"))
}
