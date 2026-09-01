package aibom

import "testing"

func TestMetricSegmentsTrend(t *testing.T) {
	cases := []struct {
		name        string
		first, last *float64
		want        string
	}{
		{"rising", f(10), f(90), "up"},
		{"falling", f(90), f(10), "down"},
		{"steady", f(50), f(52), "flat"},
		{"zero to zero", f(0), f(0), "flat"},
		{"zero to nonzero", f(0), f(5), "up"},
		{"missing first", nil, f(50), ""},
		{"missing last", f(50), nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seg := MetricSegments{FirstThird: c.first, LastThird: c.last}
			if got := seg.Trend(); got != c.want {
				t.Fatalf("Trend() = %q, want %q", got, c.want)
			}
		})
	}
}
