package aibom

import "encoding/json"

import "testing"

func TestFlexIntFromNumber(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`8`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f != 8 {
		t.Fatalf("expected 8, got %d", f)
	}
}

func TestFlexIntFromString(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`"8"`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f != 8 {
		t.Fatalf("expected 8, got %d", f)
	}
}

func TestFlexIntFromMalformedString(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`"Not available"`), &f); err != nil {
		t.Fatalf("expected no error decoding a non-numeric string, got %v", err)
	}
	if f != 0 {
		t.Fatalf("expected zero value for unparseable string, got %d", f)
	}
}

func TestEnvironmentCPUCoresAsString(t *testing.T) {
	// Regression test: real clusters emit environment.cpu_cores as a JSON
	// string (grep -c stdout, never int()-cast in generate_snapshot.py).
	var env Environment
	raw := []byte(`{"cpu_cores": "32", "gpu_count": "4", "numa_nodes": "2"}`)
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.CPUCores != 32 || env.GPUCount != 4 || env.NUMANodes != 2 {
		t.Fatalf("unexpected environment: %+v", env)
	}
}

func TestFlexFloatFromString(t *testing.T) {
	var f FlexFloat
	if err := json.Unmarshal([]byte(`"3e-4"`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f != 3e-4 {
		t.Fatalf("expected 3e-4, got %v", f)
	}
}

func TestFlexFloatFromNumber(t *testing.T) {
	var f FlexFloat
	if err := json.Unmarshal([]byte(`0.9`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f != 0.9 {
		t.Fatalf("expected 0.9, got %v", f)
	}
}
