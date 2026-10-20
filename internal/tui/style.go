package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// The palette is a groovebox front panel: dusty print on a dark case,
// orange and pink keys, small status LEDs.
var (
	dusty  = lipgloss.Color("#D9B3A6") // body text
	faded  = lipgloss.Color("#8E7F7A") // hints and secondary text
	orange = lipgloss.Color("#F28A3B") // page titles, cursor, current step
	pink   = lipgloss.Color("#E0577F") // selected option, logo
	led    = lipgloss.Color("#86C7A5") // done
	amber  = lipgloss.Color("#E8B44A") // warnings, sudo
	red    = lipgloss.Color("#E5534B") // failures
)

var (
	text      = lipgloss.NewStyle().Foreground(dusty)
	strong    = text.Bold(true)
	dim       = lipgloss.NewStyle().Foreground(faded)
	accent    = lipgloss.NewStyle().Foreground(orange)
	selected  = lipgloss.NewStyle().Foreground(pink).Bold(true)
	pageTitle = lipgloss.NewStyle().Foreground(orange).Bold(true)
	good      = lipgloss.NewStyle().Foreground(led)
	bad       = lipgloss.NewStyle().Foreground(red)
	warning   = lipgloss.NewStyle().Foreground(amber)
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

// logo is the header: "PROTEA THEME" lit like a groovebox's LED matrix,
// two pixel rows per line of half blocks, shaded from orange to pink.
var logo = drawLogo("PROTEA THEME")

func drawLogo(word string) string {
	var pixels [5]string
	for i, r := range word {
		for y, row := range glyphs[r] {
			if i > 0 {
				pixels[y] += "."
			}
			pixels[y] += row
		}
	}
	width := len(pixels[0])
	shades := lipgloss.Blend1D(width, orange, pink)

	var lines []string
	for y := 0; y < len(pixels); y += 2 {
		var line strings.Builder
		for x := range width {
			top := pixels[y][x] == '#'
			bottom := y+1 < len(pixels) && pixels[y+1][x] == '#'
			switch {
			case top && bottom:
				line.WriteString(lipgloss.NewStyle().Foreground(shades[x]).Render("█"))
			case top:
				line.WriteString(lipgloss.NewStyle().Foreground(shades[x]).Render("▀"))
			case bottom:
				line.WriteString(lipgloss.NewStyle().Foreground(shades[x]).Render("▄"))
			default:
				line.WriteString(" ")
			}
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
