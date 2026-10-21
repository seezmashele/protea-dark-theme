package system

// setting is one appearance setting Protea provides, which uninstalling
// switches away from.
type setting struct {
	file, group, key string // where Plasma stores it
	unset            string // Plasma's value when the key isn't set
	fallback         string // what to switch back to
	apply            func(r *Runner, value string) error
}

var (
	lookAndFeel = setting{
		file: "kdeglobals", group: "KDE", key: "LookAndFeelPackage",
		unset: "org.kde.breeze.desktop", fallback: "org.kde.breezedark.desktop",
		// Applying a Global Theme is how System Settings replaces one. Plasma
		// writes the theme's values to ~/.config/kdedefaults and drops the
		// user's own values for them, so this resets every part the Global
		// Theme covers and leaves nothing pointing at Protea Dark.
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-lookandfeel", "--apply", v) },
	}
	colorScheme = setting{
		file: "kdeglobals", group: "General", key: "ColorScheme",
		unset: "BreezeLight", fallback: "BreezeDark",
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-colorscheme", v) },
	}
	plasmaStyle = setting{
		file: "plasmarc", group: "Theme", key: "name",
		unset: "default", fallback: "default",
		apply: func(r *Runner, v string) error { return r.change("plasma-apply-desktoptheme", v) },
	}
	widgetStyle = setting{
		file: "kdeglobals", group: "KDE", key: "widgetStyle",
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
		file: "kwinrc", group: "org.kde.kdecoration2", key: "theme",
		unset: "Breeze", fallback: "Breeze",
		// KWin's own tool: checks the theme exists, saves it and reloads KWin.
		apply: func(r *Runner, v string) error { return r.change(libexec("kwin-applywindowdecoration"), v) },
	}
	// buttonShape is the Protea window decoration's own "Button shape"
	// setting (cpp/kdecoration/proteasettingsdata.kcfg).
	buttonShape = setting{
		file: "protearc", group: "Windeco", key: "ButtonShape",
		unset: ButtonsRectangle, fallback: ButtonsRectangle,
		apply: func(r *Runner, v string) error {
			if err := r.writeConfig("protearc", "Windeco", "ButtonShape", v); err != nil {
				return err
			}
			// What System Settings sends after a decoration's settings
			// change: KWin reloads them, so a running Protea decoration
			// switches straight away.
			return r.change("dbus-send", "--session", "--type=signal", "/KWin", "org.kde.KWin.reloadConfig")
		},
	}
)

// The window decoration's button shapes, as its "Button shape" setting
// stores them.
const (
	ButtonsRound     = "ButtonRound"
	ButtonsRectangle = "ButtonRectangle"
)

// SetButtonShape sets the Protea window decoration's button shape,
// ButtonsRound or ButtonsRectangle.
func (r *Runner) SetButtonShape(shape string) error { return buttonShape.apply(r, shape) }

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

// switchAway changes s to its Breeze fallback if Protea's value is selected,
// so removing Protea's files never leaves Plasma pointing at them. Settings
// the user has changed to something else are left alone. It reports
// whether it switched.
func (r *Runner) switchAway(s *setting, protea string) (bool, error) {
	if s.read() != protea {
		return false, nil
	}
	return true, s.apply(r, s.fallback)
}
