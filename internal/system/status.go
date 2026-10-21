package system

import (
	"os"
	"path/filepath"
	"strings"
)

// Status is a snapshot of the user's setup.
type Status struct {
	Plasma string // running Plasma version, e.g. "6.8.0"

	CoreInstalled     bool
	Accent            string // the installed color scheme's accent, #rrggbb
	TaskStyle         string // the installed task manager style, TasksUnderline or TasksBackground; "" if unknown
	TabStyle          string // how installed widgets mark the selected tab, TabsNone or TabsUnderline; "" if unknown
	GlobalThemeActive bool
	ColorsActive      bool
	PlasmaStyleActive bool

	Package          string            // installed protea package version, "" if none
	Build            map[string]string // the package's build-info
	StyleActive      bool
	DecorationActive bool
	ButtonShape      string // the window decoration's button shape, ButtonsRound or ButtonsRectangle

	MissingTools []string

	Kdecoration        string // installed kdecoration version
	KdecorationUpgrade string // version a pending upgrade would install, "" if none
}

// BuiltStyle reports whether the installed package has the application style.
func (s Status) BuiltStyle() bool { return s.Build["WITH_STYLE"] == "ON" }

// BuiltDecoration reports whether the installed package has the window decoration.
func (s Status) BuiltDecoration() bool { return s.Build["WITH_DECORATIONS"] == "ON" }

// BuiltFor is the Plasma feature release the decoration was built against.
func (s Status) BuiltFor() string { return Feature(s.Build["KDECORATION_VERSION"]) }

// Mismatch reports a decoration built for a different Plasma feature release
// than the installed one. The package's version pin should make this
// impossible; it is checked anyway before switching to the decoration.
func (s Status) Mismatch() bool {
	return s.BuiltDecoration() && s.Kdecoration != "" && s.BuiltFor() != Feature(s.Kdecoration)
}

// UpgradeBlocked reports a pending Plasma feature upgrade that the
// decoration's version pin blocks.
func (s Status) UpgradeBlocked() bool {
	return s.Package != "" && s.BuiltDecoration() && s.KdecorationUpgrade != "" &&
		Feature(s.KdecorationUpgrade) != Feature(s.Kdecoration)
}

// PlasmaSupported reports whether the running Plasma is the release Protea targets.
func (s Status) PlasmaSupported() bool { return Feature(s.Plasma) == TargetPlasma }

// coreFiles maps each embedded core theme path to where it is installed,
// relative to DataHome.
var coreFiles = []struct{ src, dest string }{
	{"theme/look-and-feel/" + GlobalThemeID, "plasma/look-and-feel/" + GlobalThemeID},
	{"theme/desktoptheme/" + PlasmaStyleID, "plasma/desktoptheme/" + PlasmaStyleID},
	{"theme/color-schemes/" + ColorSchemeID + ".colors", "color-schemes/" + ColorSchemeID + ".colors"},
}

// A Variant is a Plasma Style file that comes in more than one style, chosen
// in the installer. The Plasma Style's own copy is the default style, and
// the others replace it. The script that draws them marks each file with its
// style in a <desc> element, which is how the installed style is told apart.
type Variant struct {
	file    string            // relative to the Plasma Style's folder
	mark    string            // id of the <desc> holding the style
	Default string            // the style of the Plasma Style's own copy
	others  map[string]string // the other styles' embedded files
}

// How the task manager marks open apps: a line under each one, or a tint
// behind each one. Drawn by scripts/task-frames.py.
const (
	TasksUnderline  = "underline"
	TasksBackground = "background"
)

// How widgets mark the selected tab: not at all, or a short line under it.
// Drawn by scripts/widget-frames.py.
const (
	TabsNone      = "none"
	TabsUnderline = "underline"
)

var (
	TaskStyles = Variant{file: "widgets/tasks.svg", mark: "protea-task-style", Default: TasksUnderline,
		others: map[string]string{TasksBackground: "theme/variants/tasks-background.svg"}}
	TabStyles = Variant{file: "widgets/tabbar.svg", mark: "protea-tab-style", Default: TabsNone,
		others: map[string]string{TabsUnderline: "theme/variants/tabbar-underline.svg"}}
)

// Known reports whether style is one of v's styles.
func (v Variant) Known(style string) bool {
	_, ok := v.others[style]
	return style == v.Default || ok
}

// path is where the file is installed.
func (v Variant) path() string {
	return filepath.Join(DataHome(), "plasma/desktoptheme", PlasmaStyleID, v.file)
}

// installed is the style the installed file is marked with, or "" if it
// isn't installed or has no mark.
func (v Variant) installed() string {
	data, err := os.ReadFile(v.path())
	if err != nil {
		return ""
	}
	_, after, ok := strings.Cut(string(data), `<desc id="`+v.mark+`">`)
	if !ok {
		return ""
	}
	style, _, _ := strings.Cut(after, "<")
	return style
}

func coreInstalled() bool {
	for _, c := range coreFiles {
		if _, err := os.Stat(filepath.Join(DataHome(), c.dest)); err != nil {
			return false
		}
	}
	return true
}

// pacmanVersion returns the installed version of pkg, or "".
func pacmanVersion(pkg string) string {
	out, err := query("pacman", "-Q", pkg)
	if err != nil {
		return ""
	}
	if _, v, ok := strings.Cut(out, " "); ok {
		return v
	}
	return ""
}

// ReadStatus inspects the system. It only reads.
func ReadStatus() Status {
	s := Status{CoreInstalled: coreInstalled(), Accent: installedAccent(),
		TaskStyle: TaskStyles.installed(), TabStyle: TabStyles.installed()}

	if out, err := query("plasmashell", "--version"); err == nil {
		s.Plasma = strings.TrimPrefix(out, "plasmashell ")
	}

	s.GlobalThemeActive = lookAndFeel.read() == GlobalThemeID
	s.ColorsActive = colorScheme.read() == ColorSchemeID
	s.PlasmaStyleActive = plasmaStyle.read() == PlasmaStyleID
	s.StyleActive = widgetStyle.read() == WidgetStyleID
	s.DecorationActive = decoration.read() == DecorationID
	s.ButtonShape = buttonShape.read()

	s.Package = pacmanVersion(PackageName)
	if data, err := os.ReadFile(BuildInfoPath); err == nil && s.Package != "" {
		s.Build = parseKeyValues(string(data))
	}

	for _, tool := range BuildTools {
		if pacmanVersion(tool) == "" {
			s.MissingTools = append(s.MissingTools, tool)
		}
	}

	s.Kdecoration = pacmanVersion("kdecoration")
	// Uses the package lists from the last `pacman -Sy`; prints
	// "kdecoration 6.8.0-1 -> 6.9.0-1" when an upgrade is pending.
	if out, err := query("pacman", "-Qu", "kdecoration"); err == nil {
		if _, v, ok := strings.Cut(out, " -> "); ok {
			s.KdecorationUpgrade = v
		}
	}
	return s
}
