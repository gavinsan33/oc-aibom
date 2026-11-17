package aibom

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// go test ./internal/aibom -run CycloneDX -update rewrites the samples;
// copy them to aibom-webhook-service's docs/examples/cyclonedx/ after.
var updateSamples = flag.Bool("update", false, "rewrite testdata/cyclonedx samples")

// loadRecord decodes a fixture the way the dynamic client does (integers as
// int64), so the test sees the types a live cluster hands back.
func loadRecord(t *testing.T, name string) AIBOM {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cyclonedx", name))
	if err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	a, err := fromUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// withGit is the training record with source_code populated, since both
// real records have it null; the mapping page's third sample.
func withGit(t *testing.T) AIBOM {
	a := loadRecord(t, "aibom-training-run.json")
	a.RawData["source_code"] = map[string]any{
		"git_repository": "https://github.com/example-org/finetune-recipes.git",
		"git_commit":     "3f2a9c1d8e7b6a5f4c3d2e1f0a9b8c7d6e5f4a3b",
		"git_branch":     "main",
		"dirty":          true,
		"declared_via":   "git_directory",
	}
	// Its own object name, so its serial differs from the training sample's.
	a.Name = "aibom-granite-lora-finetune-deepspeed-gitex"
	return a
}

func convert(t *testing.T, a AIBOM) map[string]any {
	t.Helper()
	doc, err := CycloneDX(a)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(doc, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCycloneDXReproducesSamples(t *testing.T) {
	cases := map[string]func() AIBOM{
		"sample-training-run.cdx.json":          func() AIBOM { return loadRecord(t, "aibom-training-run.json") },
		"sample-serving-run.cdx.json":           func() AIBOM { return loadRecord(t, "aibom-serving-run.json") },
		"sample-training-run-with-git.cdx.json": func() AIBOM { return withGit(t) },
	}
	for sample, record := range cases {
		t.Run(sample, func(t *testing.T) {
			path := filepath.Join("testdata", "cyclonedx", sample)
			doc, err := CycloneDX(record())
			if err != nil {
				t.Fatal(err)
			}
			if *updateSamples {
				if err := os.WriteFile(path, doc, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(doc) != string(want) {
				t.Errorf("output differs from %s; rerun with -update if the mapping changed\n%s", sample, firstDiff(string(want), string(doc)))
			}
		})
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return "line " + strconv.Itoa(i+1) + ":\n  want " + wl + "\n  got  " + gl
		}
	}
	return ""
}

func props(c map[string]any) map[string]string {
	out := map[string]string{}
	for _, p := range list(c["properties"]) {
		out[str(obj(p)["name"])] = str(obj(p)["value"])
	}
	return out
}

func components(doc map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, c := range list(doc["components"]) {
		out[str(obj(c)["bom-ref"])] = obj(c)
	}
	return out
}

func TestCycloneDXServingRunSplitsModelQualityFromServerLoad(t *testing.T) {
	doc := convert(t, loadRecord(t, "aibom-serving-run.json"))
	comps := components(doc)
	model := comps["model:drawais/Granite-3.3-8B-Instruct-AWQ-INT4"]
	types := map[string]bool{}
	for _, m := range list(obj(obj(model["modelCard"])["quantitativeAnalysis"])["performanceMetrics"]) {
		types[str(obj(m)["type"])] = true
	}
	if !types["time_to_first_token_seconds:avg"] || !types["generation_throughput_tokens_per_sec:p95"] {
		t.Errorf("model card metrics = %v", types)
	}
	runtime := props(comps["runtime:vllm"])
	if runtime["aibomrun:inference:telemetry:kv_cache_usage:avg"] == "" {
		t.Error("kv-cache usage should be on the runtime component")
	}
	if _, ok := runtime["aibomrun:inference:servingEngine"]; ok {
		t.Error("serving engine is aibomrun:runtime:name, not an inference property")
	}
}

func TestCycloneDXNeverInventsValues(t *testing.T) {
	a := loadRecord(t, "aibom-training-run.json")
	obj(a.RawData["environment"])["driver_version"] = "Not available"
	obj(a.RawData["model"])["name"] = "/mnt/models"
	delete(a.RawData, "telemetry_series_ref")
	a.Signature = ""
	doc := convert(t, a)
	root := obj(obj(doc["metadata"])["component"])
	if root["purl"] != nil {
		t.Errorf("a path isn't a package, got purl %v", root["purl"])
	}
	for _, c := range components(doc) {
		if c["type"] == "device-driver" {
			t.Error("no driver component without a driver version")
		}
	}
	refs := list(root["externalReferences"])
	if len(refs) != 1 || obj(refs[0])["type"] != "attestation" {
		t.Errorf("externalReferences = %v", refs)
	}
	for name := range props(root) {
		if strings.HasPrefix(name, "aibomrun:signature:") {
			t.Errorf("unsigned record got %s", name)
		}
	}
}

func TestCycloneDXUnknownIntent(t *testing.T) {
	a := loadRecord(t, "aibom-training-run.json")
	a.RawData["experiment_intent"] = "unknown"
	doc := convert(t, a)
	meta := obj(doc["metadata"])
	if meta["lifecycles"] != nil {
		t.Error("unknown intent has no lifecycle")
	}
	if obj(meta["component"])["type"] != "application" {
		t.Error("unknown intent's root is an application")
	}
	model := obj(list(doc["components"])[0])
	if model["type"] != "machine-learning-model" || props(model)["aibomrun:model:framework"] != "trl" {
		t.Errorf("first component = %v", model)
	}
}

func TestCycloneDXSerialPrefersTriggerUID(t *testing.T) {
	a := loadRecord(t, "aibom-serving-run.json")
	obj(a.RawData["execution_metadata"])["trigger"] = map[string]any{"kind": "Job", "uid": "u-1"}
	doc := convert(t, a)
	if got, want := doc["serialNumber"], "urn:uuid:"+uuid5(cdxSerialNamespace, "u-1"); got != want {
		t.Errorf("serial = %v, want %v", got, want)
	}
	if got := str(obj(list(obj(obj(doc["metadata"])["tools"])["components"])[0])["version"]); got != "" {
		t.Errorf("tool version = %q, want none for a record without generator_version", got)
	}
	obj(a.RawData["_metadata"])["generator_version"] = "0.3.0"
	if got := str(obj(list(obj(obj(convert(t, a)["metadata"])["tools"])["components"])[0])["version"]); got != "0.3.0" {
		t.Errorf("tool version = %q, want 0.3.0", got)
	}
	if got := props(obj(obj(doc["metadata"])["component"]))["aibomrun:job:triggerKind"]; got != "Job" {
		t.Errorf("triggerKind = %q, want Job", got)
	}
	// The namespace UUID is uuid5(NAMESPACE_URL, project URL).
	urlNS := [16]byte{0x6b, 0xa7, 0xb8, 0x11, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	if got := uuid5(urlNS, "https://github.com/gavinsan33/aibom-webhook-service"); got != "efb51e52-f1c6-57af-b409-621e826f2301" {
		t.Errorf("serial namespace = %s", got)
	}
}

func TestCycloneDXTypedAndLegacyEnvironmentAgree(t *testing.T) {
	legacy := convert(t, loadRecord(t, "aibom-training-run.json"))["components"]
	a := loadRecord(t, "aibom-training-run.json")
	env := obj(a.RawData["environment"])
	env["cpu_cores"] = int64(512)
	obj(env["network"])["rdma_devices"] = []any{"mlx5_0", "mlx5_1", "mlx5_2", "mlx5_3", "mlx5_4", "mlx5_5"}
	obj(env["storage"])["block_devices"] = []any{
		map[string]any{"name": "nvme0n1", "size": "894.3G"},
		map[string]any{"name": "nvme1n1", "size": "7T"},
	}
	// Components only: the attestation hash covers the record, which changed.
	if typed := convert(t, a)["components"]; !reflect.DeepEqual(typed, legacy) {
		t.Error("typed and legacy environment values give different components")
	}
}

func TestCycloneDXContainers(t *testing.T) {
	a := loadRecord(t, "aibom-serving-run.json")
	digest := "sha256:" + strings.Repeat("a", 64)
	bare := "sha256:" + strings.Repeat("b", 64)
	pod := obj(list(obj(a.RawData["execution_metadata"])["pods"])[0])
	pod["containers"] = []any{
		map[string]any{"name": "server", "image": "quay.io/org/vllm:v1", "image_id": "quay.io/org/vllm@" + digest},
		map[string]any{"name": "client", "image": "quay.io/org/vllm:v1", "image_id": "quay.io/org/vllm@" + digest},
		map[string]any{"name": "bare", "image": "busybox", "image_id": bare},
	}
	comps := components(convert(t, a))
	vllm := comps["image:"+digest]
	if vllm["name"] != "quay.io/org/vllm" || vllm["version"] != "v1" {
		t.Errorf("vllm image = %v", vllm)
	}
	if want := "pkg:oci/vllm@" + digest + "?repository_url=quay.io/org/vllm"; vllm["purl"] != want {
		t.Errorf("purl = %v, want %v", vllm["purl"], want)
	}
	if got := props(vllm)["aibomrun:container:names"]; got != "server,client" {
		t.Errorf("container names = %q", got)
	}
	b := comps["image:"+bare]
	if b["purl"] != nil || props(b)["aibomrun:container:imageId"] != bare {
		t.Errorf("bare config digest = %v", b)
	}
}

func TestFormatNumber(t *testing.T) {
	for f, want := range map[float64]string{
		607: "607", 1793723397: "1793723397", 0.0002: "0.0002", 1.33e-05: "1.33e-05",
		1e-08: "1e-08", 1511.38: "1511.38", 0: "0", -2.5: "-2.5",
	} {
		if got := formatNumber(f); got != want {
			t.Errorf("formatNumber(%v) = %q, want %q", f, got, want)
		}
	}
}
