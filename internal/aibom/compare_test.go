package aibom

import "testing"

func findField(rows []FieldRow, label string) (FieldRow, bool) {
	for _, r := range rows {
		if r.Label == label {
			return r, true
		}
	}
	return FieldRow{}, false
}

func TestCompareFieldsIdentical(t *testing.T) {
	a := AIBOM{JobName: "job-1", Data: Data{Model: Model{Name: "granite", Version: "1.0"}}}
	rows := CompareFields([]AIBOM{a, a})
	for _, r := range rows {
		if r.Differs {
			t.Fatalf("identical AIBOMs should not differ on %q (%v)", r.Label, r.Values)
		}
	}
}

func TestCompareFieldsModelVersion(t *testing.T) {
	a := AIBOM{Data: Data{Model: Model{Name: "granite", Version: "1.0"}}}
	b := AIBOM{Data: Data{Model: Model{Name: "granite", Version: "2.0"}}}
	rows := CompareFields([]AIBOM{a, b})
	if r, _ := findField(rows, "Model Version"); !r.Differs || r.Values[0] != "1.0" || r.Values[1] != "2.0" {
		t.Fatalf("Model Version row = %+v, want differs 1.0/2.0", r)
	}
	if r, _ := findField(rows, "Model Name"); r.Differs {
		t.Fatalf("Model Name should not differ, got %+v", r)
	}
}

func TestCompareFieldsDatasetDrift(t *testing.T) {
	a := AIBOM{Data: Data{Dataset: Dataset{AutoDetected: []AutoDetectedDataset{{MatchesDeclared: true}}}}}
	b := AIBOM{Data: Data{Dataset: Dataset{AutoDetected: []AutoDetectedDataset{{MatchesDeclared: false}}}}}
	r, ok := findField(CompareFields([]AIBOM{a, b}), "Dataset Drift")
	if !ok || !r.Differs || r.Values[0] != "false" || r.Values[1] != "true" {
		t.Fatalf("Dataset Drift row = %+v, want differs false/true", r)
	}
}

func TestCompareFieldsThreeWayDiffers(t *testing.T) {
	a := AIBOM{Data: Data{Environment: Environment{GPUType: "A100"}}}
	b := AIBOM{Data: Data{Environment: Environment{GPUType: "A100"}}}
	c := AIBOM{Data: Data{Environment: Environment{GPUType: "H100"}}}
	r, _ := findField(CompareFields([]AIBOM{a, b, c}), "GPU Type")
	if !r.Differs || len(r.Values) != 3 {
		t.Fatalf("GPU Type across 3 runs = %+v, want differs with 3 values", r)
	}
}

func TestComparePerformanceDeltaAndPctChange(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_utilization": {Avg: 50, Unit: "percent"}}}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_utilization": {Avg: 75}}}}}
	rows := ComparePerformance([]AIBOM{a, b}, []string{"gpu_utilization"})
	r := rows[0]
	if r.Values[0] != 50 || r.Values[1] != 75 || r.Delta == nil || *r.Delta != 25 {
		t.Fatalf("delta row = %+v, want 50/75 delta 25", r)
	}
	if r.PctChange == nil || *r.PctChange != 50 {
		t.Fatalf("pct change = %v, want +50", r.PctChange)
	}
	if r.Unit != "percent" {
		t.Fatalf("unit = %q, want percent (first non-empty)", r.Unit)
	}
}

func TestComparePerformanceZeroBaselineNoPct(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_power": {Avg: 0}}}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"gpu_power": {Avg: 100}}}}}
	r := ComparePerformance([]AIBOM{a, b}, []string{"gpu_power"})[0]
	if r.PctChange != nil {
		t.Fatalf("zero baseline should give nil pct change, got %v", *r.PctChange)
	}
	if r.Delta == nil || *r.Delta != 100 {
		t.Fatalf("delta = %v, want 100", r.Delta)
	}
}

func TestComparePerformanceNoDeltaBeyondTwoRuns(t *testing.T) {
	a := AIBOM{}
	r := ComparePerformance([]AIBOM{a, a, a}, []string{"gpu_utilization"})[0]
	if r.Delta != nil || r.PctChange != nil {
		t.Fatalf("three-run compare should not set delta/pct, got %+v", r)
	}
}

func TestComparePerformanceUnavailableOnNote(t *testing.T) {
	a := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Note: "telemetry unavailable"}}}
	b := AIBOM{Data: Data{ResourceUtilization: ResourceUtilization{Metrics: map[string]MetricStats{"cpu_usage": {Avg: 2}}}}}
	r := ComparePerformance([]AIBOM{a, b}, []string{"cpu_usage"})[0]
	if r.Available[0] || !r.Available[1] {
		t.Fatalf("availability = %v, want [false true]", r.Available)
	}
}

func TestCompareInferencePerformance(t *testing.T) {
	// a has vLLM serving metrics; b has no inference block at all.
	a := AIBOM{Data: Data{Inference: &Inference{Performance: &InferencePerformance{
		Metrics: map[string]MetricStats{"generation_throughput": {Avg: 120, Unit: "tokens/s"}},
	}}}}
	b := AIBOM{}
	r := CompareInferencePerformance([]AIBOM{a, b}, []string{"generation_throughput"})[0]
	if r.Values[0] != 120 || r.Unit != "tokens/s" {
		t.Fatalf("row = %+v, want value 120 unit tokens/s", r)
	}
	if !r.Available[0] || r.Available[1] {
		t.Fatalf("availability = %v, want [true false]", r.Available)
	}
}
