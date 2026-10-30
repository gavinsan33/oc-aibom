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
