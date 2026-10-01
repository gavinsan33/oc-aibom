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
