package aibom

import (
	"encoding/json"
	"testing"
)

func TestPodStatusUnmarshal(t *testing.T) {
	raw := `{"pod_name": "job-abc", "status": "OOMKilled", "exit_code": 137}`
	var p Pod
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Status != "OOMKilled" {
		t.Errorf("Status = %q, want OOMKilled", p.Status)
	}
	if p.ExitCode == nil || *p.ExitCode != 137 {
		t.Errorf("ExitCode = %v, want 137", p.ExitCode)
	}
}

func TestPodStatusUnmarshalNull(t *testing.T) {
	raw := `{"pod_name": "job-abc", "status": null, "exit_code": null}`
	var p Pod
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Status != "" {
		t.Errorf("Status = %q, want empty", p.Status)
	}
	if p.ExitCode != nil {
		t.Errorf("ExitCode = %v, want nil", p.ExitCode)
	}
}

func TestMetricStatsUnmarshalWithLimit(t *testing.T) {
	raw := `{"unit": "GB", "min": 2, "max": 8, "avg": 5, "p95": 7.8, "limit": 8, "segments": {}}`
	var m MetricStats
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Limit == nil || *m.Limit != 8 {
		t.Errorf("Limit = %v, want 8", m.Limit)
	}
}

func TestMetricStatsUnmarshalNoLimit(t *testing.T) {
	raw := `{"unit": "percent", "min": 10, "max": 95, "avg": 60, "p95": 94, "limit": null, "segments": {}}`
	var m MetricStats
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Limit != nil {
		t.Errorf("Limit = %v, want nil", m.Limit)
	}
}

func TestDataExperimentIntentDeclaredViaUnmarshal(t *testing.T) {
	raw := `{"experiment_intent": "inference", "experiment_intent_declared_via": "inferred_from_model_detection"}`
	var d Data
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.ExperimentIntentDeclaredVia != "inferred_from_model_detection" {
		t.Errorf("ExperimentIntentDeclaredVia = %q, want inferred_from_model_detection", d.ExperimentIntentDeclaredVia)
	}
}

func TestDataExperimentIntentDeclaredViaAbsentOnOlderAIBOM(t *testing.T) {
	raw := `{"experiment_intent": "training"}`
	var d Data
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.ExperimentIntentDeclaredVia != "" {
		t.Errorf("ExperimentIntentDeclaredVia = %q, want empty on an AIBOM predating this field", d.ExperimentIntentDeclaredVia)
	}
}

func TestInferencePerformanceUnmarshal(t *testing.T) {
	raw := `{
		"serving_engine": "vllm",
		"performance": {
			"collected_at": "2026-01-01T00:00:00Z",
			"summary_includes_cold_start": true,
			"metrics": {
				"time_to_first_token_seconds": {"unit": "seconds", "min": 0.1, "max": 0.5, "avg": 0.3, "p95": 0.45, "limit": null, "segments": {}}
			}
		}
	}`
	var inf Inference
	if err := json.Unmarshal([]byte(raw), &inf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if inf.Performance == nil {
		t.Fatal("Performance = nil, want non-nil")
	}
	if !inf.Performance.SummaryIncludesColdStart {
		t.Error("SummaryIncludesColdStart = false, want true")
	}
	m, ok := inf.Performance.Metrics["time_to_first_token_seconds"]
	if !ok {
		t.Fatal("Metrics[\"time_to_first_token_seconds\"] missing")
	}
	if m.Avg != 0.3 {
		t.Errorf("Avg = %v, want 0.3", m.Avg)
	}
}

func TestInferencePerformanceNilOnOlderAIBOM(t *testing.T) {
	raw := `{"serving_engine": "vllm", "max_model_len": 10000}`
	var inf Inference
	if err := json.Unmarshal([]byte(raw), &inf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if inf.Performance != nil {
		t.Errorf("Performance = %+v, want nil on an AIBOM predating this field", inf.Performance)
	}
}
