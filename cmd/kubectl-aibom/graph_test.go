package main

import (
	"bytes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"math"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func TestSparkline(t *testing.T) {
	pts := [][2]float64{{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}, {5, 5}, {6, 6}, {7, 7}}
	if got := sparkline(pts, 8, 0, 7); got != "▁▂▃▄▅▆▇█" {
		t.Fatalf("ramp: %q", got)
	}
	if got := sparkline(pts, 2, 0, 7); got != "▂▆" { // cell averages 1.5 and 5.5
		t.Fatalf("downsampled: %q", got)
	}
	if got := sparkline([][2]float64{{0, 5}, {1, 5}}, 4, 5, 5); got != "▅▅▅▅" {
		t.Fatalf("flat/stretched: %q", got)
	}
	// Shared scale: a run peaking at 1 against a shared max of 8 stays low.
	if got := sparkline([][2]float64{{0, 1}}, 1, 0, 8); got != "▁" {
		t.Fatalf("shared scale: %q", got)
	}
}

func TestPrintGraph(t *testing.T) {
	mk := func(start, end int64, gib float64) aibom.Series {
		var s aibom.Series
		s.Window.Start, s.Window.End = start, end
		s.Metrics = map[string]aibom.SeriesMetric{
			"memory_usage": {Unit: "bytes", Aggregate: [][2]float64{{float64(start), gib * (1 << 30)}, {float64(end), gib * (1 << 30)}}},
		}
		return s
	}
	runs := []graphRun{{"short", mk(1000, 1060, 1)}, {"long", mk(5000, 5120, 2)}}
	var buf bytes.Buffer
	if err := printGraph(&buf, runs, nil, false, 40); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "2m0s") || !strings.Contains(out, "max 2.00 GiB") || !strings.Contains(out, "max 1.00 GiB") {
		t.Fatalf("output: %s", out)
	}
	// short run is half the longest, so about half as many cells.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "short ") && utf8.RuneCountInString(line) > 30 {
			t.Fatalf("short run not scaled to shared axis: %q", line)
		}
	}
	if err := printGraph(&buf, runs, []string{"nope"}, false, 40); err == nil {
		t.Fatal("expected unknown-metric error")
	}
}

func tuiRuns() []graphRun {
	mk := func(dur int64, amp float64) aibom.Series {
		var s aibom.Series
		s.Window.Start, s.Window.End = 1000, 1000+dur
		var pts [][2]float64
		for i := 0; i < 60; i++ {
			pts = append(pts, [2]float64{float64(1000 + int64(i)*dur/60), amp * (0.5 + 0.5*math.Sin(float64(i)/6))})
		}
		s.Metrics = map[string]aibom.SeriesMetric{
			"gpu_utilization": {Unit: "percent", Aggregate: pts},
			"gpu_power":       {Unit: "watts", Aggregate: pts},
		}
		return s
	}
	return []graphRun{{"run1", mk(3600, 90)}, {"run2", mk(1800, 60)}}
}

func TestGraphModelView(t *testing.T) {
	runs := tuiRuns()
	keys, _ := graphMetricKeys(runs, nil)
	var m tea.Model = newGraphModel(runs, keys, false)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	out := m.View()
	if os.Getenv("SHOW_TUI") != "" {
		t.Log("\n" + out)
	}
	for _, want := range []string{"GPU Utilization", "GPU Power", "run1", "run2", "1h00m", "avg"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view missing %q", want)
		}
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.(graphModel); got.sel != 1 || !got.zoom {
		t.Fatalf("sel %d zoom %v", got.sel, got.zoom)
	}
	if strings.Contains(m.View(), "GPU Utilization\n") {
		t.Fatal("zoomed view should show only the selected metric")
	}
}

func TestGraphModelOverlapAndHide(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Two identical runs: every dot coincides, so the plot is all overlap color.
	r := tuiRuns()[0]
	runs := []graphRun{{"a", r.Series}, {"b", r.Series}}
	keys, _ := graphMetricKeys(runs, nil)
	var m tea.Model = newGraphModel(runs, keys, false)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	box := func() string { // just the first chart panel, so legend/footer colors don't count
		v := m.View()
		return v[strings.Index(v, "╭"):strings.Index(v, "╯")]
	}
	if out := box(); !strings.Contains(out, "\x1b[97m") || strings.Contains(out, "\x1b[34m") || strings.Contains(out, "\x1b[35m") {
		t.Fatalf("expected only overlap color on coincident lines")
	}

	// Hiding run 1 leaves only run 2's own color.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if out := box(); !strings.Contains(out, "\x1b[35m") || strings.Contains(out, "\x1b[97m") {
		t.Fatalf("expected run 2's color and no overlap color after hiding run 1")
	}
}

func TestGraphModelQuitBacksOutOfZoom(t *testing.T) {
	runs := tuiRuns()
	keys, _ := graphMetricKeys(runs, nil)
	var m tea.Model = newGraphModel(runs, keys, false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	m, cmd := m.Update(q)
	if cmd != nil || m.(graphModel).zoom {
		t.Fatal("q while zoomed should only zoom out")
	}
	if _, cmd = m.Update(q); cmd == nil {
		t.Fatal("q while not zoomed should quit")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("ctrl+c should quit even when zoomed")
	}
}
