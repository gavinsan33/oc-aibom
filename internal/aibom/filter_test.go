package aibom

import "testing"

func newTestAIBOMs() []AIBOM {
	return []AIBOM{
		{
			Name:             "run-a",
			JobName:          "job-a",
			ExperimentIntent: "training",
			Data: Data{
				Model:       Model{Name: "granite-3.0-8b", Quantization: "none", Architecture: "gpt", Framework: "pytorch"},
				SourceCode:  SourceCode{GitBranch: "main", GitRepository: "org/repo-a"},
				Environment: Environment{GPUType: "A100"},
				Training:    &Training{Optimizer: "adamw"},
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
			JobName:          "job-b",
			ExperimentIntent: "sft",
			Data: Data{
				Model:       Model{Name: "granite-3.0-8b", Quantization: "int4", Architecture: "gpt", Framework: "pytorch"},
				SourceCode:  SourceCode{GitBranch: "feature/x", GitRepository: "org/repo-a"},
				Environment: Environment{GPUType: "H100"},
				FineTuning:  &FineTuning{AdaptationMethod: "lora"},
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
			JobName:          "job-c",
			ExperimentIntent: "inference",
			Data: Data{
				Model:       Model{Name: "llama-3-70b", Quantization: "int8", Architecture: "llama"},
				Environment: Environment{GPUType: "A100"},
				Inference:   &Inference{ServingEngine: "vllm"},
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

func TestApplyFilterByGPUType(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{GPUType: "a100"})
	if len(items) != 2 {
		t.Fatalf("expected 2 items on A100, got %d: %+v", len(items), items)
	}
}

func TestApplyFilterByArchitecture(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Architecture: "llama"})
	if len(items) != 1 || items[0].Name != "run-c" {
		t.Fatalf("expected only run-c, got %+v", items)
	}
}

func TestApplyFilterByJobName(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{JobName: "job-b"})
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b, got %+v", items)
	}
}

func TestApplyFilterByGitBranch(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{GitBranch: "feature/x"})
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b, got %+v", items)
	}
}

func TestApplyFilterByServingEngine(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{ServingEngine: "vllm"})
	if len(items) != 1 || items[0].Name != "run-c" {
		t.Fatalf("expected only run-c, got %+v", items)
	}
}

func TestApplyFilterByServingEngineSkipsNilInference(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{ServingEngine: "does-not-exist"})
	if len(items) != 0 {
		t.Fatalf("expected no matches, got %+v", items)
	}
}

func TestApplyFilterByAdaptationMethod(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{AdaptationMethod: "lora"})
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b, got %+v", items)
	}
}

func TestApplyFilterByOptimizer(t *testing.T) {
	items := Apply(newTestAIBOMs(), Filter{Optimizer: "adamw"})
	if len(items) != 1 || items[0].Name != "run-a" {
		t.Fatalf("expected only run-a, got %+v", items)
	}
}

func TestDriftOnly(t *testing.T) {
	items := DriftOnly(newTestAIBOMs())
	if len(items) != 1 || items[0].Name != "run-b" {
		t.Fatalf("expected only run-b to have drift, got %+v", items)
	}
}
