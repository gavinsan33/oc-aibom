package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline draws values as unicode blocks across exactly width cells
// (averaging when there are more points than cells, repeating when fewer, so
// runs of different lengths stay aligned on a shared time axis). Heights are
// scaled to lo..hi, which callers share across runs so lines are comparable;
// lo==hi renders mid-height.
// ponytail: buckets by point index, so gaps in the series aren't shown as
// gaps; fine at ~200 downsampled points, revisit if series get sparse.
func sparkline(pts [][2]float64, width int, lo, hi float64) string {
	n := len(pts)
	if n == 0 || width < 1 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		l, h := i*n/width, (i+1)*n/width
		if h <= l {
			h = l + 1
		}
		var sum float64
		for _, p := range pts[l:h] {
			sum += p[1]
		}
		v := sum / float64(h-l)
		idx := len(sparkBlocks) / 2
		if hi > lo {
			idx = int((v - lo) / (hi - lo) * float64(len(sparkBlocks)-1))
		}
		b.WriteRune(sparkBlocks[max(0, min(idx, len(sparkBlocks)-1))])
	}
	return b.String()
}

// fmtSeriesValue scales the series' raw base units to the display units
// describe uses.
func fmtSeriesValue(unit string, v float64) string {
	switch unit {
	case "bytes":
		return fmt.Sprintf("%.2f GiB", v/(1<<30))
	case "bytes_per_sec":
		return fmt.Sprintf("%.2f MB/s", v/1e6)
	case "seconds":
		return fmt.Sprintf("%.0f ms", v*1000)
	case "percent":
		return fmt.Sprintf("%.1f %%", v)
	case "watts":
		return fmt.Sprintf("%.0f W", v)
	default:
		return fmt.Sprintf("%.2f %s", v, strings.ReplaceAll(unit, "_", "/"))
	}
}

func seriesStats(pts [][2]float64) (min, avg, max float64) {
	min, max = pts[0][1], pts[0][1]
	for _, p := range pts {
		if p[1] < min {
			min = p[1]
		}
		if p[1] > max {
			max = p[1]
		}
		avg += p[1]
	}
	return min, avg / float64(len(pts)), max
}

// graphMetricKeys is the metrics to draw, in display order: every metric any
// run has (hardware, then inference, then unknown ones sorted), or only the
// named ones, which must exist in at least one run.
func graphMetricKeys(runs []graphRun, only []string) ([]string, error) {
	order := append(append([]string{}, telemetryMetricOrder...), vllmMetricOrder...)
	known := map[string]bool{}
	for _, k := range order {
		known[k] = true
	}
	present := map[string]bool{}
	var extra []string
	for _, r := range runs {
		for k := range r.Series.Metrics {
			if !present[k] && !known[k] {
				extra = append(extra, k)
			}
			present[k] = true
		}
	}
	sort.Strings(extra)
	order = append(order, extra...)

	if len(only) > 0 {
		for _, k := range only {
			if !present[k] {
				avail := make([]string, 0, len(present))
				for m := range present {
					avail = append(avail, m)
				}
				sort.Strings(avail)
				return nil, fmt.Errorf("no series for metric %q; available: %s", k, strings.Join(avail, ", "))
			}
		}
		return only, nil
	}
	keys := order[:0:0]
	for _, k := range order {
		if present[k] {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func graphMetricLabel(k string) string {
	if l := telemetryMetricLabels[k]; l != "" {
		return l
	}
	if l := vllmMetricLabels[k]; l != "" {
		return l
	}
	return k
}

// graphRun is one AIBOM's loaded series, in the order given on the command
// line (which fixes its color, as in compare).
type graphRun struct {
	Name   string
	Series aibom.Series
}

type graphRow struct {
	label, peak string
	color       int
	pts         [][2]float64
	dur, start  int64
}

// sharedElapsed is the longest run's duration in seconds, the shared x range
// (the plugin's sharedElapsedMax).
func sharedElapsed(runs []graphRun) int64 {
	var m int64 = 1
	for _, r := range runs {
		m = max(m, r.Series.Window.End-r.Series.Window.Start)
	}
	return m
}

// graphRows is the lines to draw for one metric: one aggregate row per run
// that has it, or with perSeries (and more than one, undropped series) that
// run's pod/GPU/container rows instead.
func graphRows(runs []graphRun, k string, perSeries bool) (rows []graphRow, unit string) {
	for i, r := range runs {
		dur := r.Series.Window.End - r.Series.Window.Start
		m, ok := r.Series.Metrics[k]
		if !ok || len(m.Aggregate) == 0 {
			continue
		}
		unit = m.Unit
		if perSeries && !m.SeriesOmitted && len(m.Series) > 1 {
			for _, sr := range m.Series {
				if len(sr.Points) == 0 {
					continue
				}
				var parts []string
				for _, lk := range []string{"pod", "container", "interface", "gpu"} {
					if v := sr.Labels[lk]; v != "" {
						parts = append(parts, v)
					}
				}
				rows = append(rows, graphRow{label: r.Name + " · " + strings.Join(parts, "/"), color: i, pts: sr.Points, dur: dur, start: r.Series.Window.Start})
			}
			continue
		}
		row := graphRow{label: r.Name, color: i, pts: m.Aggregate, dur: dur, start: r.Series.Window.Start}
		if len(m.AggregateMax) > 0 { // bucket peaks survive downsampling; the avg line can hide spikes
			_, _, pk := seriesStats(m.AggregateMax)
			row.peak = fmt.Sprintf("  peak %s", fmtSeriesValue(m.Unit, pk))
		}
		rows = append(rows, row)
	}
	return rows, unit
}

// printGraph draws one block per metric with a row per run, mirroring the
// console plugin's compare charts: x is elapsed time since each run's own
// start on a shared axis (a shorter run is a shorter line) and the y scale is
// shared across the runs so heights are comparable. With perSeries, a run's
// pod/GPU/container lines replace its aggregate line, as the plugin's
// "Show individual pods and GPUs" does.
func printGraph(w io.Writer, runs []graphRun, only []string, perSeries bool, width int) error {
	order, err := graphMetricKeys(runs, only)
	if err != nil {
		return err
	}

	sharedMax := sharedElapsed(runs)
	fmt.Fprintf(w, "Elapsed time since each run's start; the axis spans 0 → %s (longest run).\n\n", time.Duration(sharedMax)*time.Second)

	for _, k := range order {
		rows, unit := graphRows(runs, k, perSeries)
		if len(rows) == 0 {
			continue
		}

		lo, hi, labelW := rows[0].pts[0][1], rows[0].pts[0][1], 0
		for _, row := range rows {
			rmin, _, rmax := seriesStats(row.pts)
			lo, hi = min(lo, rmin), max(hi, rmax)
			labelW = max(labelW, utf8.RuneCountInString(row.label))
		}
		label := graphMetricLabel(k)
		fmt.Fprintln(w, cyan(label))
		avail := max(10, width-labelW-2)
		for _, row := range rows {
			cells := max(1, min(avail, int(float64(avail)*float64(row.dur)/float64(sharedMax)+0.5)))
			pad := strings.Repeat(" ", labelW-utf8.RuneCountInString(row.label))
			rmin, ravg, rmax := seriesStats(row.pts)
			fmt.Fprintf(w, "  %s%s %s\n", colorize(runColor(row.color), row.label), pad, colorize(runColor(row.color), sparkline(row.pts, cells, lo, hi)))
			fmt.Fprintf(w, "  %s min %s  avg %s  max %s%s\n", strings.Repeat(" ", labelW), fmtSeriesValue(unit, rmin), fmtSeriesValue(unit, ravg), fmtSeriesValue(unit, rmax), row.peak)
		}
		fmt.Fprintln(w)
	}
	return nil
}
