package system

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// setting is one appearance setting Protea changes.
type setting struct {
	name             string // key in the saved-settings file
	file, group, key string // where Plasma stores it
	unset            string // Plasma's value when the key isn't set
	fallback         string // what to switch back to when nothing was saved
	apply            func(r *Runner, value string) error
}

var (
	lookAndFeel = setting{
		name: "lookAndFeel", file: "kdeglobals", group: "KDE", key: "LookAndFeelPackage",
		unset: "org.kde.breeze.desktop", fallback: "org.kde.breezedark.desktop",
		// Only the record of the active Global Theme; its parts are applied
		// one by one through the other settings.
		apply: func(r *Runner, v string) error { return r.writeConfig("kdeglobals", "KDE", "LookAndFeelPackage", v) },
	}
	colorScheme = setting{
		name: "colorScheme", file: "kdeglobals", group: "General", key: "ColorScheme",
		unset: "BreezeLight", fallback: "BreezeDark",
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-colorscheme", v) },
	}
	plasmaStyle = setting{
		name: "plasmaStyle", file: "plasmarc", group: "Theme", key: "name",
		unset: "default", fallback: "default",
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-desktoptheme", v) },
	}
	iconTheme = setting{
		name: "iconTheme", file: "kdeglobals", group: "Icons", key: "Theme",
		unset: "breeze", fallback: "breeze-dark",
		apply: func(r *Runner, v string) error { return r.change(libexec("plasma-changeicons"), v) },
	}
	cursorTheme = setting{
		name: "cursorTheme", file: "kcminputrc", group: "Mouse", key: "cursorTheme",
		unset: "breeze_cursors", fallback: "breeze_cursors",
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-cursortheme", v) },
	}
	widgetStyle = setting{
		name: "widgetStyle", file: "kdeglobals", group: "KDE", key: "widgetStyle",
		unset: "Breeze", fallback: "Breeze",
		apply: func(r *Runner, v string) error {
			if err := r.writeConfig("kdeglobals", "KDE", "widgetStyle", v); err != nil {
				return err
			}
			// What System Settings sends: KGlobalSettings StyleChanged (2),
			// so running apps switch style.
			return r.change("dbus-send", "--session", "--type=signal", "/KGlobalSettings",
				"org.kde.KGlobalSettings.notifyChange", "int32:2", "int32:0")
		},
	}
	decoration = setting{
		name: "decoration", file: "kwinrc", group: "org.kde.kdecoration2", key: "theme",
		unset: "Breeze", fallback: "Breeze",
		// KWin's own tool: checks the theme exists, saves it and reloads KWin.
		apply: func(r *Runner, v string) error { return r.change(libexec("kwin-applywindowdecoration"), v) },
	}
)

// themeValues are the settings Protea Dark's theme files provide.
var themeValues = map[*setting]string{
	&lookAndFeel: GlobalThemeID,
	&colorScheme: ColorSchemeID,
	&plasmaStyle: PlasmaStyleID,
}

// globalThemeDefaults are the values Protea Dark's Global Theme sets for
// settings it doesn't own. Keep in sync with its contents/defaults.
var globalThemeDefaults = map[*setting]string{
	&iconTheme:   "breeze-dark",
	&cursorTheme: "breeze_cursors",
	&widgetStyle: "Breeze",
	&decoration:  "Breeze",
}

func (r *Runner) writeConfig(file, group, key, value string) error {
	return r.change("kwriteconfig6", "--file", file, "--group", group, "--key", key, value)
}

func (s *setting) read() string {
	v, err := query("kreadconfig6", "--file", s.file, "--group", s.group, "--key", s.key)
	if err != nil || v == "" {
		return s.unset
	}
	return v
}

// The saved-settings file keeps each setting's value from before Protea
// changed it, so switching back restores the user's own choice.
func savedPath() string { return filepath.Join(stateHome(), "protea", "saved-settings") }

func loadSaved() map[string]string {
	data, err := os.ReadFile(savedPath())
	if err != nil {
		return map[string]string{}
	}
	return parseKeyValues(string(data))
}

func (r *Runner) storeSaved(saved map[string]string) error {
	if r.DryRun {
		return nil
	}
	if len(saved) == 0 {
		err := os.Remove(savedPath())
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	keys := make([]string, 0, len(saved))
	for k := range saved {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + saved[k] + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(savedPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(savedPath(), []byte(b.String()), 0o644)
}

// remember records s's current value before Protea changes it to value,
// unless a value is already saved or s is already value.
func remember(saved map[string]string, s *setting, value string) {
	if _, ok := saved[s.name]; ok {
		return
	}
	if current := s.read(); current != value {
		saved[s.name] = current
	}
}

// set changes s to value, remembering the current value first.
func (r *Runner) set(s *setting, value string) error {
	saved := loadSaved()
	remember(saved, s, value)
	if err := r.storeSaved(saved); err != nil {
		return err
	}
	return s.apply(r, value)
}

// restore switches s back to its saved value, or to its fallback when
// nothing was saved and needSaved is false, if its current value is one of
// ours. A saved value is used up when it is restored; if the user has since
// picked something else, it is kept and their choice is left alone.
func (r *Runner) restore(s *setting, needSaved bool, ours ...string) error {
	saved := loadSaved()
	previous, wasSaved := saved[s.name]
	current := s.read()
	isOurs := false
	for _, o := range ours {
		isOurs = isOurs || current == o
	}
	if wasSaved && isOurs {
		delete(saved, s.name)
		if err := r.storeSaved(saved); err != nil {
			return err
		}
	}
	if !isOurs || (!wasSaved && needSaved) {
		return nil
	}
	if !wasSaved {
		previous = s.fallback
	}
	if previous == current {
		return nil
	}
	return s.apply(r, previous)
}
