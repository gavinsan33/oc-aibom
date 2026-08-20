package aibom

import "testing"

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
