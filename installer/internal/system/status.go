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
	GlobalThemeActive bool
	ColorsActive      bool
	PlasmaStyleActive bool

	Package          string            // installed protea package version, "" if none
	Build            map[string]string // the package's build-info
	StyleActive      bool
	DecorationActive bool

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
	s := Status{CoreInstalled: coreInstalled()}

	if out, err := query("plasmashell", "--version"); err == nil {
		s.Plasma = strings.TrimPrefix(out, "plasmashell ")
	}

	s.GlobalThemeActive = lookAndFeel.read() == GlobalThemeID
	s.ColorsActive = colorScheme.read() == ColorSchemeID
	s.PlasmaStyleActive = plasmaStyle.read() == PlasmaStyleID
	s.StyleActive = widgetStyle.read() == WidgetStyleID
	s.DecorationActive = decoration.read() == DecorationID

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
