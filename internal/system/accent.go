package system

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Accent is an accent color the installer offers.
type Accent struct {
	Name string
	Hex  string
}

// Accents are the installer's accent colors. The first is the default.
var Accents = []Accent{
	{"Teal", "#5ba9ae"},
	{"Protea pink", "#e0577f"},
	{"Orange", "#f28a3b"},
	{"Amber", "#e8b44a"},
	{"Sage", "#8fb573"},
	{"Lavender", "#9b8cd9"},
	{"Breeze blue", "#3daee9"},
}

// DefaultAccent is Protea Dark's accent color.
var DefaultAccent = Accents[0].Hex

// AccentName returns the name of a preset accent, or its hex value.
func AccentName(hex string) string {
	for _, a := range Accents {
		if strings.EqualFold(a.Hex, hex) {
			return a.Name
		}
	}
	return hex
}

// rgb is a color with channels from 0 to 255.
type rgb [3]float64

func parseHex(hex string) (rgb, error) {
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil || len(strings.TrimPrefix(hex, "#")) != 6 {
		return rgb{}, fmt.Errorf("not a #rrggbb color: %q", hex)
	}
	return rgb{float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff)}, nil
}

// parseKDE reads a color as KDE writes it: "r,g,b".
func parseKDE(s string) (rgb, bool) {
	parts := strings.Split(s, ",")
	if len(parts) < 3 {
		return rgb{}, false
	}
	var c rgb
	for i := range c {
		v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return rgb{}, false
		}
		c[i] = float64(v)
	}
	return c, true
}

func (c rgb) kde() string {
	return fmt.Sprintf("%d,%d,%d", int(math.Round(c[0])), int(math.Round(c[1])), int(math.Round(c[2])))
}

func (c rgb) hex() string {
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(c[0])), int(math.Round(c[1])), int(math.Round(c[2])))
}

// luminance is WCAG relative luminance.
func (c rgb) luminance() float64 {
	var l [3]float64
	for i, v := range c {
		v /= 255
		if v <= 0.04045 {
			l[i] = v / 12.92
		} else {
			l[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
}

// contrast is the WCAG contrast ratio of two colors.
func contrast(a, b rgb) float64 {
	la, lb := a.luminance(), b.luminance()
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// mix returns t of a blended over 1-t of b.
func mix(a, b rgb, t float64) rgb {
	var c rgb
	for i := range c {
		c[i] = t*a[i] + (1-t)*b[i]
	}
	return c
}

var (
	white = rgb{252, 252, 252} // Breeze's text white
	black = rgb{0, 0, 0}
)

// The rules below follow System Settings' accent colors (plasma-workspace
// kcms/colors/colorsapplicator), without tinting or accent title bars.

// readableOn is accent as link text on bg: lightened or darkened until the
// contrast ratio reaches 5, like System Settings does.
func readableOn(accent, bg rgb) rgb {
	towards := black
	if accent.luminance() > bg.luminance() {
		towards = white
	}
	c := accent
	for i := 0; i < 10 && contrast(c, bg) < 5; i++ {
		c = mix(towards, c, 0.15)
	}
	return c
}

// accentBackground is the accent at 70% over bg, used for selections.
func accentBackground(accent, bg rgb) rgb { return mix(accent, bg, 0.7) }

// accentForeground is readable text on an accent background.
func accentForeground(bg rgb) rgb {
	if contrast(bg, white) < 4.5 {
		return black
	}
	return white
}

// viewBackground is Protea Dark's view background, for previews.
var viewBackground = rgb{20, 22, 24}

// AccentPreview returns, as #rrggbb, the selection background, the text on
// it, and the link color the accent gives on Protea Dark's views.
func AccentPreview(accentHex string) (selection, selectionText, link string, err error) {
	accent, err := parseHex(accentHex)
	if err != nil {
		return "", "", "", err
	}
	bg := accentBackground(accent, viewBackground)
	return bg.hex(), accentForeground(bg).hex(), readableOn(accent, viewBackground).hex(), nil
}

// ApplyAccent rewrites the accent colors in a KDE color file: a color
// scheme, or a Plasma Style's colors. Only keys already in the file
// change, so keys a Plasma Style leaves out keep following the system
// color scheme, as Plasma intends.
func ApplyAccent(text, accentHex string) (string, error) {
	accent, err := parseHex(accentHex)
	if err != nil {
		return "", err
	}

	// First pass: every group's colors, to blend against.
	colors := map[string]map[string]rgb{}
	group := ""
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "[") {
			group = line
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			if c, ok := parseKDE(v); ok {
				if colors[group] == nil {
					colors[group] = map[string]rgb{}
				}
				colors[group][k] = c
			}
		}
	}
	background := func(g string) rgb {
		if c, ok := colors[g]["BackgroundNormal"]; ok {
			return c
		}
		return colors["[Colors:View]"]["BackgroundNormal"]
	}
	selectionBg := accentBackground(accent, background("[Colors:View]"))
	selectionFg := accentForeground(selectionBg)

	// The new value for a key in a group, if the accent sets it.
	value := func(g, key string) (rgb, bool) {
		if !strings.HasPrefix(g, "[Colors:") {
			return rgb{}, false
		}
		selection := g == "[Colors:Selection]"
		switch {
		case key == "DecorationFocus" || key == "DecorationHover":
			return accent, true
		case key == "ForegroundActive" && selection:
			return selectionFg, true
		case key == "ForegroundActive":
			return accent, true
		case key == "ForegroundLink" && !selection:
			return readableOn(accent, background(g)), true
		case selection && (key == "BackgroundNormal" || key == "BackgroundAlternate"):
			return selectionBg, true
		case selection && (key == "ForegroundNormal" || key == "ForegroundInactive"):
			return selectionFg, true
		case key == "BackgroundAlternate" && (g == "[Colors:Button]" || g == "[Colors:Complementary]"):
			return accentBackground(accent, background(g)), true
		}
		return rgb{}, false
	}

	// Second pass: rewrite those keys, keeping everything else as it is.
	var out strings.Builder
	group = ""
	sc = bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "[") {
			group = line
		} else if k, _, ok := strings.Cut(line, "="); ok {
			if c, ok := value(group, k); ok {
				line = k + "=" + c.kde()
			}
		}
		out.WriteString(line + "\n")
	}
	return out.String(), nil
}

// accentFiles are the installed color files the accent is written into,
// relative to DataHome.
var accentFiles = []string{
	"color-schemes/" + ColorSchemeID + ".colors",
	"plasma/desktoptheme/" + PlasmaStyleID + "/colors",
}

func (r *Runner) writeAccent(accentHex string) error {
	if r.DryRun {
		r.logf("would set the accent color to %s (%s)", AccentName(accentHex), accentHex)
		return nil
	}
	for _, rel := range accentFiles {
		path := filepath.Join(DataHome(), rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated, err := ApplyAccent(string(data), accentHex)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// installedAccent is the accent of the installed color scheme, or "".
func installedAccent() string {
	data, err := os.ReadFile(filepath.Join(DataHome(), accentFiles[0]))
	if err != nil {
		return ""
	}
	group := ""
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "[") {
			group = line
		} else if k, v, ok := strings.Cut(line, "="); ok && group == "[Colors:View]" && k == "DecorationFocus" {
			if c, ok := parseKDE(v); ok {
				return c.hex()
			}
		}
	}
	return ""
}
