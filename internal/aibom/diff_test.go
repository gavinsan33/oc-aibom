package aibom

import (
	"math"
	"testing"
)

func findDiff(diffs []FieldDiff, field string) (FieldDiff, bool) {
	for _, d := range diffs {
		if d.Field == field {
			return d, true
		}
	}
	return FieldDiff{}, false
}

func TestDiffIdentical(t *testing.T) {
	a := AIBOM{JobName: "job-1", Data: Data{Model: Model{Name: "granite", Version: "1.0"}}}
	b := a
	diffs := Diff(a, b)
	if len(diffs) != 0 {
		t.Fatalf("expected no diffs for identical AIBOMs, got %+v", diffs)
	}
}

func TestDiffModelVersion(t *testing.T) {
	a := AIBOM{Data: Data{Model: Model{Name: "granite", Version: "1.0"}}}
	b := AIBOM{Data: Data{Model: Model{Name: "granite", Version: "2.0"}}}
	diffs := Diff(a, b)
	d, ok := findDiff(diffs, "model.version")
	if !ok {
		t.Fatalf("expected model.version diff, got %+v", diffs)
	}
	if d.A != "1.0" || d.B != "2.0" {
		t.Fatalf("unexpected diff values: %+v", d)
	}
	if _, ok := findDiff(diffs, "model.name"); ok {
		t.Fatalf("model.name should not differ, got %+v", diffs)
	}
}

func TestDiffDatasetDrift(t *testing.T) {
	a := AIBOM{Data: Data{
		Dataset: Dataset{AutoDetected: []AutoDetectedDataset{{MatchesDeclared: true}}},
	}}
	b := AIBOM{Data: Data{
		Dataset: Dataset{AutoDetected: []AutoDetectedDataset{{MatchesDeclared: false}}},
	}}
	d, ok := findDiff(Diff(a, b), "dataset.drift")
	if !ok {
		t.Fatalf("expected dataset.drift diff")
	}
	if d.A != "false" || d.B != "true" {
		t.Fatalf("unexpected drift diff values: %+v", d)
	}
}

func TestDiffEnvironment(t *testing.T) {
	a := AIBOM{Data: Data{Environment: Environment{GPUType: "A100", GPUCount: 4}}}
	b := AIBOM{Data: Data{Environment: Environment{GPUType: "H100", GPUCount: 8}}}
	diffs := Diff(a, b)
	if _, ok := findDiff(diffs, "environment.gpu_type"); !ok {
		t.Fatalf("expected environment.gpu_type diff, got %+v", diffs)
	}
	if _, ok := findDiff(diffs, "environment.gpu_count"); !ok {
		t.Fatalf("expected environment.gpu_count diff, got %+v", diffs)
	}
}

func findMetric(diffs []MetricDiff, metric string) (MetricDiff, bool) {
	for _, d := range diffs {
		if d.Metric == metric {
			return d, true
		}
	}
	return MetricDiff{}, false
}

func TestDiffPerformanceComputesDeltaAndPctChange(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUUtilizationPct: 50}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUUtilizationPct: 75}}}
	m, ok := findMetric(DiffPerformance(a, b), "avg_gpu_utilization_pct")
	if !ok {
		t.Fatalf("expected avg_gpu_utilization_pct metric")
	}
	if m.A != 50 || m.B != 75 || m.Delta != 25 {
		t.Fatalf("unexpected metric: %+v", m)
	}
	if m.PctChange != 50 {
		t.Fatalf("expected +50%% change, got %v", m.PctChange)
	}
}

func TestDiffPerformanceZeroBaselineIsNaN(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUPowerWatts: 0}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{AvgGPUPowerWatts: 100}}}
	m, ok := findMetric(DiffPerformance(a, b), "avg_gpu_power_watts")
	if !ok {
		t.Fatalf("expected avg_gpu_power_watts metric")
	}
	if !math.IsNaN(m.PctChange) {
		t.Fatalf("expected NaN pct change with zero baseline, got %v", m.PctChange)
	}
	if m.Delta != 100 {
		t.Fatalf("expected delta 100, got %v", m.Delta)
	}
}

func TestDiffPerformanceReturnsAllMetricsEvenIfUnchanged(t *testing.T) {
	a := AIBOM{}
	b := AIBOM{}
	metrics := DiffPerformance(a, b)
	if len(metrics) != len(performanceMetrics) {
		t.Fatalf("expected %d metrics, got %d", len(performanceMetrics), len(metrics))
	}
}

func f(v float64) *float64 { return &v }

func TestDiffPerformanceCarriesPerRunTrend(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{
		AvgGPUUtilizationPct: 50,
		Metrics: map[string]MetricStats{
			"gpu_utilization": {Segments: MetricSegments{FirstThird: f(90), LastThird: f(10)}},
		},
	}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{
		AvgGPUUtilizationPct: 50,
		Metrics: map[string]MetricStats{
			"gpu_utilization": {Segments: MetricSegments{FirstThird: f(10), LastThird: f(90)}},
		},
	}}}
	m, ok := findMetric(DiffPerformance(a, b), "avg_gpu_utilization_pct")
	if !ok {
		t.Fatalf("expected avg_gpu_utilization_pct metric")
	}
	if m.TrendA != "down" {
		t.Fatalf("expected TrendA=down, got %q", m.TrendA)
	}
	if m.TrendB != "up" {
		t.Fatalf("expected TrendB=up, got %q", m.TrendB)
	}
}

func TestDiffPerformanceTrendEmptyWhenMetricMissing(t *testing.T) {
	a := AIBOM{}
	b := AIBOM{}
	m, ok := findMetric(DiffPerformance(a, b), "avg_gpu_utilization_pct")
	if !ok {
		t.Fatalf("expected avg_gpu_utilization_pct metric")
	}
	if m.TrendA != "" || m.TrendB != "" {
		t.Fatalf("expected empty trends with no Metrics data, got %q / %q", m.TrendA, m.TrendB)
	}
}
