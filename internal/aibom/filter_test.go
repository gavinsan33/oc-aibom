package aibom

import "testing"

func newTestAIBOMs() []AIBOM {
	return []AIBOM{
		{
			Name:             "run-a",
			ExperimentIntent: "training",
			Data: Data{
				Model: Model{Name: "granite-3.0-8b", Quantization: "none"},
				Dataset: Dataset{
					Declared: DeclaredDataset{Name: "alpaca", Version: "1.0", License: "Apache-2.0"},
					AutoDetected: []AutoDetectedDataset{
						{DatasetName: "alpaca", Version: "1.0", MatchesDeclared: true},
					},
				},
			},
		},
		{
			Name:             "run-b",
			ExperimentIntent: "sft",
			Data: Data{
				Model: Model{Name: "granite-3.0-8b", Quantization: "int4"},
				Dataset: Dataset{
					Declared: DeclaredDataset{Name: "alpaca", Version: "1.0", License: "Apache-2.0"},
					AutoDetected: []AutoDetectedDataset{
						{DatasetName: "dolly", Version: "2.0", MatchesDeclared: false},
					},
				},
			},
		},
		{
			Name:             "run-c",
			ExperimentIntent: "inference",
			Data: Data{
				Model: Model{Name: "llama-3-70b", Quantization: "int8"},
			},
		},
	}
}

func TestApplyFilterByModel(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Model: "granite-3.0-8b"})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}
}

func TestApplyFilterCaseInsensitive(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Model: "GRANITE-3.0-8B"})
	if len(items) != 2 {
		t.Fatalf("expected case-insensitive match to find 2 items, got %d", len(items))
	}
}

func TestApplyFilterByIntent(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Intent: "inference"})
	if len(items) != 1 || items[0].Name != "run-c" {
		t.Fatalf("expected only run-c, got %+v", items)
	}
}

func TestApplyFilterByQuantization(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Quantization: "int4"})
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b, got %+v", items)
	}
}

func TestApplyFilterCombined(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Model: "granite-3.0-8b", Intent: "sft"})
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b, got %+v", items)
	}
}

func TestApplyNoFilterReturnsAll(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{})
	if len(items) != 3 {
		t.Fatalf("expected all 3 items with no filter, got %d", len(items))
	}
}

func TestDriftOnly(t *testing.T) {
	items := DriftOnly(newTestAIBOMs())
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b to have drift, got %+v", items)
	}
}
