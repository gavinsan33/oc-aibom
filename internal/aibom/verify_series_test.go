package aibom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const testSeriesJSON = `{"metrics":{"cpu_usage":{"aggregate":[[1767225600,1.5]],"aggregation":"sum","unit":"cores"}},"schema_version":1,"window":{"end":1767229200,"start":1767225600,"step_seconds":30}}`

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newTelemetryObject(namespace, name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "aibom.io/v1alpha1",
		"kind":       "AIBOMTelemetry",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec":       spec,
	}}
}

func aibomWithSeriesRef(ref map[string]any) AIBOM {
	data := map[string]any{"model": map[string]any{"name": "x"}}
	if ref != nil {
		data["telemetry_series_ref"] = ref
	}
	return AIBOM{Name: "run-1", Namespace: "ml-a", RawData: data}
}

func goodRef() map[string]any {
	return map[string]any{
		"schema_version": int64(1),
		"kind":           "AIBOMTelemetry",
		"name":           "run-telemetry-ab12cd34",
		"sha256":         sha256Hex(testSeriesJSON),
		"size_bytes":     int64(len(testSeriesJSON)),
	}
}

var validAIBOM = VerifyResult{Status: VerifyValid}

func TestVerifySeries_Absent(t *testing.T) {
	result := VerifySeries(context.Background(), nil, aibomWithSeriesRef(nil), validAIBOM)
	if result.Status != SeriesAbsent {
		t.Fatalf("expected SeriesAbsent, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_VerifiedWhenDigestMatchesAndAIBOMIsValid(t *testing.T) {
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesVerified {
		t.Fatalf("expected SeriesVerified, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_DigestOnlyWhenAIBOMSignatureNotValid(t *testing.T) {
	// A matching digest inside an AIBOM whose signature can't be trusted proves
	// nothing about the digest itself -- never report it as Verified.
	for _, status := range []VerifyStatus{VerifyUnsigned, VerifyUnconfirmed, VerifyKeyMismatch, VerifyInvalid} {
		client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
		result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), VerifyResult{Status: status})
		if result.Status != SeriesDigestOnly {
			t.Fatalf("AIBOM status %v: expected SeriesDigestOnly, got %v (%s)", status, result.Status, result.Detail)
		}
	}
}

func TestVerifySeries_MismatchWhenContentAltered(t *testing.T) {
	tampered := strings.Replace(testSeriesJSON, "1.5", "0.1", 1)
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": tampered}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesMismatch {
		t.Fatalf("expected SeriesMismatch, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_MismatchWhenSizeDisagreesEvenIfDigestMatches(t *testing.T) {
	ref := goodRef()
	ref["size_bytes"] = int64(len(testSeriesJSON) + 1)
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(ref), validAIBOM)
	if result.Status != SeriesMismatch {
		t.Fatalf("expected SeriesMismatch, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_SizeAsFloatFromJSONDecodeStillMatches(t *testing.T) {
	ref := goodRef()
	ref["size_bytes"] = float64(len(testSeriesJSON))
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(ref), validAIBOM)
	if result.Status != SeriesVerified {
		t.Fatalf("expected SeriesVerified, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_DigestComparisonIgnoresHexCase(t *testing.T) {
	ref := goodRef()
	ref["sha256"] = strings.ToUpper(sha256Hex(testSeriesJSON))
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(ref), validAIBOM)
	if result.Status != SeriesVerified {
		t.Fatalf("expected SeriesVerified, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_MismatchWhenObjectHasNoSeriesJSON(t *testing.T) {
	client := newTestDynamicClient(newTelemetryObject("ml-a", "run-telemetry-ab12cd34", map[string]any{"schemaVersion": int64(1)}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesMismatch {
		t.Fatalf("expected SeriesMismatch, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_MissingWhenObjectGone(t *testing.T) {
	client := newTestDynamicClient()
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesMissing {
		t.Fatalf("expected SeriesMissing, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_ObjectInAnotherNamespaceIsNotUsed(t *testing.T) {
	client := newTestDynamicClient(newTelemetryObject("other-ns", "run-telemetry-ab12cd34", map[string]any{"seriesJson": testSeriesJSON}))
	result := VerifySeries(context.Background(), client, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesMissing {
		t.Fatalf("expected SeriesMissing, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_UnconfirmedWithNoCluster(t *testing.T) {
	result := VerifySeries(context.Background(), nil, aibomWithSeriesRef(goodRef()), validAIBOM)
	if result.Status != SeriesUnconfirmed {
		t.Fatalf("expected SeriesUnconfirmed, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerifySeries_UnconfirmedForUnrecognizedOrIncompleteRefs(t *testing.T) {
	cases := map[string]map[string]any{
		"legacy configmap form": {"configmap": "x-telemetry-1", "key": "series.json", "sha256": "abc"},
		"missing sha256":        {"kind": "AIBOMTelemetry", "name": "n"},
		"missing name":          {"kind": "AIBOMTelemetry", "sha256": "abc"},
	}
	for label, ref := range cases {
		result := VerifySeries(context.Background(), newTestDynamicClient(), aibomWithSeriesRef(ref), validAIBOM)
		if result.Status != SeriesUnconfirmed {
			t.Fatalf("%s: expected SeriesUnconfirmed, got %v (%s)", label, result.Status, result.Detail)
		}
	}
}
