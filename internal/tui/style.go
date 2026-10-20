package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// The palette is Base16: the terminal's own 16 ANSI colors, mapped as in
// Charm's Huh Base16 theme, so the installer follows the terminal's theme.
var (
	white       = lipgloss.Color("7") // body text
	brightBlack = lipgloss.Color("8") // hints and secondary text, end of the logo
	cyan        = lipgloss.Color("6") // page titles, logo
	yellow      = lipgloss.Color("3") // cursor, current step, warnings
	green       = lipgloss.Color("2") // selected option, done
	magenta     = lipgloss.Color("5") // page title marker
	brightRed   = lipgloss.Color("9") // failures
)

var (
	text      = lipgloss.NewStyle().Foreground(white)
	strong    = text.Bold(true)
	dim       = lipgloss.NewStyle().Foreground(brightBlack)
	accent    = lipgloss.NewStyle().Foreground(yellow)
	selected  = lipgloss.NewStyle().Foreground(green).Bold(true)
	pageTitle = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	marker    = lipgloss.NewStyle().Foreground(magenta)
	good      = lipgloss.NewStyle().Foreground(green)
	bad       = lipgloss.NewStyle().Foreground(brightRed)
	warning   = lipgloss.NewStyle().Foreground(yellow)
)

// glyphs is a 5-pixel-tall font for the logo; '#' is a lit pixel.
var glyphs = map[rune][5]string{
	'P': {"###.", "#..#", "###.", "#...", "#..."},
	'R': {"###.", "#..#", "###.", "#.#.", "#..#"},
	'O': {".##.", "#..#", "#..#", "#..#", ".##."},
	'T': {"###", ".#.", ".#.", ".#.", ".#."},
	'E': {"####", "#...", "###.", "#...", "####"},
	'A': {".##.", "#..#", "####", "#..#", "#..#"},
	'H': {"#..#", "#..#", "####", "#..#", "#..#"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#"},
	' ': {"..", "..", "..", "..", ".."},
}

// logoColors color the logo's letters in order: "PRO" in the terminal's
// cyan, "TEA" in faint cyan, and the space and "THEME" in the faded hint
// color.
var logoColors = []struct {
	letters int
	style   lipgloss.Style
}{
	{3, lipgloss.NewStyle().Foreground(cyan)},
	{3, lipgloss.NewStyle().Foreground(cyan).Faint(true)},
	{6, lipgloss.NewStyle().Foreground(brightBlack)},
}

// logo is the header: "PROTEA THEME" lit like an LED matrix, two pixel
// rows per line of half blocks, colored letter by letter by logoColors.
var logo = drawLogo("PROTEA THEME")

func drawLogo(word string) string {
	var pixels [5]string
	var styles []lipgloss.Style // each pixel column's color
	group, left := 0, logoColors[0].letters
	for i, r := range word {
		if left == 0 {
			group++
			left = logoColors[group].letters
		}
		left--
		glyph := glyphs[r]
		columns := len(glyph[0])
		for y, row := range glyph {
			if i > 0 {
				pixels[y] += "."
			}
			pixels[y] += row
		}
		if i > 0 {
			columns++ // the gap before the letter
		}
		for range columns {
			styles = append(styles, logoColors[group].style)
		}
	}

	var lines []string
	for y := 0; y < len(pixels); y += 2 {
		var line strings.Builder
		for x := range len(pixels[0]) {
			lit := styles[x]
			top := pixels[y][x] == '#'
			bottom := y+1 < len(pixels) && pixels[y+1][x] == '#'
			switch {
			case top && bottom:
				line.WriteString(lit.Render("█"))
			case top:
				line.WriteString(lit.Render("▀"))
			case bottom:
				line.WriteString(lit.Render("▄"))
			default:
				line.WriteString(" ")
			}
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
