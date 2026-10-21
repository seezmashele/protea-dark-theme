package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/fs"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"proteadesktop.org/protea-theme/internal/system"
)

func TestEmbeddedFiles(t *testing.T) {
	for _, p := range []string{
		"theme/look-and-feel/" + system.GlobalThemeID + "/metadata.json",
		"theme/desktoptheme/" + system.PlasmaStyleID + "/metadata.json",
		"theme/color-schemes/" + system.ColorSchemeID + ".colors",
		"theme/variants/tasks-background.svg",
		"cpp/CMakeLists.txt",
		"cpp/build-info.in",
		"cpp/kstyle/proteastyle.cpp",
		"cpp/po",
		"packaging/arch/PKGBUILD",
	} {
		if _, err := fs.Stat(files, p); err != nil {
			t.Errorf("not embedded: %v", err)
		}
	}
	entries, _ := fs.ReadDir(files, "cpp")
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "build") {
			t.Errorf("a local build folder was embedded: cpp/%s", e.Name())
		}
	}
}

// Windows and Plasma's panels, popups and tooltips have the same rounded
// corners: the decoration's radius is in cpp/kdecoration/protea.h, and
// scripts/frame-radius.py redraws the Plasma Style's frames.
func TestCornerRadiusMatches(t *testing.T) {
	repo := os.DirFS(".")
	h, err := fs.ReadFile(repo, "cpp/kdecoration/protea.h")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`Frame_FrameRadius = ([0-9.]+);`).FindSubmatch(h)
	if m == nil {
		t.Fatal("no Frame_FrameRadius in protea.h")
	}
	units, _ := strconv.ParseFloat(string(m[1]), 64)
	want := units * 2 // KWin's small spacing is always 2 px on Wayland

	// The files scripts/frame-radius.py redraws (FRAMES there).
	for _, f := range []string{
		"dialogs/background.svg",
		"translucent/dialogs/background.svg",
		"solid/dialogs/background.svg",
		"widgets/panel-background.svg",
		"translucent/widgets/panel-background.svg",
		"solid/widgets/panel-background.svg",
		"widgets/tooltip.svg",
		"translucent/widgets/tooltip.svg",
		"solid/widgets/tooltip.svg",
		"widgets/plasmoidheading.svg",
	} {
		data, err := fs.ReadFile(repo, "theme/desktoptheme/"+system.PlasmaStyleID+"/"+f)
		if err != nil {
			t.Fatal(err)
		}
		corner := "topleft"
		if strings.HasSuffix(f, "plasmoidheading.svg") {
			corner = "header-topleft"
		}
		got, err := cornerSize(data, corner)
		if err != nil {
			t.Errorf("%s: %v (redraw it with scripts/frame-radius.py)", f, err)
		} else if math.Abs(got-want) > 0.01 {
			t.Errorf("%s: %g px corners, the window decoration has %g px; run scripts/frame-radius.py %g", f, got, want, want)
		}
	}
}

// The task manager styles' frames, drawn by scripts/task-frames.py.
var taskStyles = map[string]string{
	"underline":  "theme/desktoptheme/" + system.PlasmaStyleID + "/widgets/tasks.svg",
	"background": "theme/variants/tasks-background.svg",
}

// When the pointer is over an app, the task manager looks for
// "<state>-hover" before "<edge>-hover" (taskPrefixHovered in TaskTools.js),
// so each plain "<state>-hover" frame drawn for bottom panels needs a copy
// for every edge with its own hover frame, or top and side panels get the
// bottom panel's hover.
func TestTaskHoverFramesPerEdge(t *testing.T) {
	for style, f := range taskStyles {
		data, err := fs.ReadFile(os.DirFS("."), f)
		if err != nil {
			t.Fatal(err)
		}
		// The installer tells the styles apart by this mark.
		if mark := `<desc id="protea-task-style">` + style + `</desc>`; !bytes.Contains(data, []byte(mark)) {
			t.Errorf("%s: no %s; run scripts/task-frames.py", f, mark)
		}
		ids := map[string]bool{}
		for _, m := range regexp.MustCompile(` id="([^"]+)"`).FindAllSubmatch(data, -1) {
			ids[string(m[1])] = true
		}
		pieces := []string{"topleft", "top", "topright", "left", "center", "right", "bottomleft", "bottom", "bottomright"}
		for _, state := range []string{"normal", "focus", "minimized", "attention", "launcher"} {
			if !ids[state+"-hover-center"] {
				continue
			}
			for _, edge := range []string{"north", "east", "west"} {
				if !ids[edge+"-hover-center"] {
					continue
				}
				for _, p := range pieces {
					if id := edge + "-" + state + "-hover-" + p; !ids[id] {
						t.Errorf("%s has %s-hover but no %s; run scripts/task-frames.py", f, state, id)
					}
				}
			}
		}
	}
}

// cornerSize is the width of a corner piece as scripts/frame-radius.py draws
// it: a path of absolute M, L and A commands inside the group with that id.
func cornerSize(svg []byte, id string) (float64, error) {
	d := xml.NewDecoder(bytes.NewReader(svg))
	inCorner := false
	for {
		tok, err := d.Token()
		if err != nil {
			return 0, fmt.Errorf("no %s piece", id)
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range el.Attr {
			if a.Name.Local == "id" && a.Value == id {
				inCorner = true
			}
		}
		if !inCorner || el.Name.Local != "path" {
			continue
		}
		for _, a := range el.Attr {
			if a.Name.Local == "d" {
				return pathWidth(a.Value)
			}
		}
	}
}

func pathWidth(d string) (float64, error) {
	fields := strings.Fields(strings.ReplaceAll(d, ",", " "))
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := 0; i < len(fields); {
		var skip int
		switch fields[i] {
		case "M", "L":
			skip = 0
		case "A":
			skip = 5 // radii, rotation and flags, then the end point
		case "Z":
			i++
			continue
		default:
			return 0, fmt.Errorf("unexpected path command %q", fields[i])
		}
		if i+skip+2 >= len(fields) {
			return 0, fmt.Errorf("short path %q", d)
		}
		x, err := strconv.ParseFloat(fields[i+1+skip], 64)
		if err != nil {
			return 0, err
		}
		lo, hi = min(lo, x), max(hi, x)
		i += skip + 3
	}
	return hi - lo, nil
}
