package aibom

import "testing"

func TestMetricSegmentsTrend(t *testing.T) {
	cases := []struct {
		name             string
		first, mid, last *float64
		want             string
	}{
		{"rising", f(10), nil, f(90), "up"},
		{"falling", f(90), nil, f(10), "down"},
		{"steady", f(50), nil, f(52), "flat"},
		{"zero to zero", f(0), nil, f(0), "flat"},
		{"zero to nonzero", f(0), nil, f(5), "up"},
		{"missing first", nil, nil, f(50), ""},
		{"missing last", f(50), nil, nil, ""},
		{"dip then recover", f(100), f(50), f(100), "volatile"},
		{"spike then drop", f(10), f(100), f(10), "volatile"},
		{"monotonic rise with middle present", f(10), f(50), f(90), "up"},
		{"monotonic fall with middle present", f(90), f(50), f(10), "down"},
		{"small wobble stays flat", f(50), f(53), f(51), "flat"},
		{"missing middle falls back to first-vs-last", f(100), nil, f(10), "down"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seg := MetricSegments{FirstThird: c.first, MiddleThird: c.mid, LastThird: c.last}
			if got := seg.Trend(); got != c.want {
				t.Fatalf("Trend() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestMetricSegmentsSparkline(t *testing.T) {
	cases := []struct {
		name             string
		first, mid, last *float64
		want             string
	}{
		{"dip then recover", f(100), f(50), f(100), "↘↗"},
		{"spike then drop", f(10), f(100), f(10), "↗↘"},
		{"steady climb", f(10), f(50), f(90), "↗↗"},
		{"steady decline", f(90), f(50), f(10), "↘↘"},
		{"flat", f(50), f(51), f(52), "→→"},
		{"missing middle", f(10), nil, f(90), ""},
		{"missing first", nil, f(50), f(90), ""},
		{"missing last", f(10), f(50), nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seg := MetricSegments{FirstThird: c.first, MiddleThird: c.mid, LastThird: c.last}
			if got := seg.Sparkline(); got != c.want {
				t.Fatalf("Sparkline() = %q, want %q", got, c.want)
			}
		})
	}
}
