package aibom

import (
	"encoding/json"
	"testing"
)

func TestModelDecodesPVCProvenanceFields(t *testing.T) {
	raw := `{"name":"Qwen/Qwen2.5-32B-Instruct","name_declared_via":"model_files_readme",
		"revision":"5ede1c97bbab6ce5cda5812749b4c0bdf79b18dd","base_model":"Qwen/Qwen2.5-32B","size_bytes":65527752704}`
	var m Model
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m.NameDeclaredVia != "model_files_readme" || m.BaseModel != "Qwen/Qwen2.5-32B" ||
		m.SizeBytes != 65527752704 || m.Revision != "5ede1c97bbab6ce5cda5812749b4c0bdf79b18dd" {
		t.Fatalf("unexpected decode: %+v", m)
	}
}

func TestEnvironmentAndInferenceDecodeSurfacedFields(t *testing.T) {
	var e Environment
	raw := `{"gpu_memory_mb":[81920,81920],"cpu":{"cpu_architecture":"x86_64"},"benchmarks":{"cpu_compute":{"mflops":"100"}}}`
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	if len(e.GPUMemoryMB) != 2 || e.CPU["cpu_architecture"] != "x86_64" || e.Benchmarks["cpu_compute"] == nil {
		t.Fatalf("unexpected decode: %+v", e)
	}
	var inf Inference
	if err := json.Unmarshal([]byte(`{"seed":0,"enforce_eager":null,"port":8000}`), &inf); err != nil {
		t.Fatal(err)
	}
	if inf.Seed == nil || inf.EnforceEager != nil || inf.Port == nil {
		t.Fatalf("seed 0 must stay set, null must stay nil: %+v", inf)
	}
}
