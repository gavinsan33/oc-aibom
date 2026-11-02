package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func describeOutput(t *testing.T, raw string) string {
	t.Helper()
	var data aibom.Data
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	printDescribe(&buf, aibom.AIBOM{Name: "a", Data: data}, aibom.VerifyResult{}, aibom.SeriesResult{}, true)
	return buf.String()
}

func TestDescribeShowsNoDatasetMessage(t *testing.T) {
	// Inference AIBOMs have no dataset block at all.
	out := describeOutput(t, `{"experiment_intent": "inference"}`)
	if !strings.Contains(out, "Dataset:\n  No dataset was detected for this workload.\n") {
		t.Fatalf("missing empty-dataset message:\n%s", out)
	}
	if strings.Contains(out, "Declared:") {
		t.Fatalf("an empty Declared line was printed:\n%s", out)
	}
}

func TestDescribeKeepsDeclaredDataset(t *testing.T) {
	out := describeOutput(t, `{"dataset": {"declared": {"name": "tatsu-lab/alpaca", "declared_via": "cli_arg"}}}`)
	if !strings.Contains(out, "Declared:      tatsu-lab/alpaca") || strings.Contains(out, "No dataset") {
		t.Fatalf("declared dataset not shown:\n%s", out)
	}
}

func TestDescribeShowsUnsetInferenceSettingsAsDash(t *testing.T) {
	// The webhook writes settings the server wasn't started with as JSON null;
	// they used to decode to 0/false and print as if they were set.
	out := describeOutput(t, `{"inference": {"serving_engine": "vllm", "max_model_len": 10000,
		"tensor_parallel_size": null, "pipeline_parallel_size": null, "data_parallel_size": null,
		"enable_expert_parallel": null, "temperature": 0.7, "top_p": 0.9, "top_k": "20", "max_tokens": null}}`)
	for _, want := range []string{
		"Max Model Len:        10000\n",
		"Tensor/Pipeline/Data Parallel: - / - / -\n",
		"Expert Parallel:      -\n",
		"Temperature/TopP/TopK: 0.7 / 0.9 / 20\n",
		"Max Tokens:           -\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDescribeKeepsZeroSettings(t *testing.T) {
	out := describeOutput(t, `{"training": {"random_seed": 0, "learning_rate": null}}`)
	if !strings.Contains(out, "Random Seed:            0\n") || !strings.Contains(out, "Learning Rate:          -\n") {
		t.Fatalf("zero vs unset not distinguished:\n%s", out)
	}
}
