package aibom

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Series is the document stored in AIBOMTelemetry.spec.seriesJson (see
// aibom-webhook-service's CLAUDE.md, "Telemetry Time Series"). Points are
// [unix seconds, value] pairs; units are raw base units.
type Series struct {
	Window struct {
		Start       int64 `json:"start"`
		End         int64 `json:"end"`
		StepSeconds int64 `json:"step_seconds"`
	} `json:"window"`
	Pods    []string                `json:"pods"`
	Metrics map[string]SeriesMetric `json:"metrics"`
}

// SeriesMetric is one metric's per-run line plus its optional per-pod/GPU
// expansion (dropped, with SeriesOmitted set, when the run had too many).
type SeriesMetric struct {
	Unit          string       `json:"unit"`
	Aggregation   string       `json:"aggregation"`
	Aggregate     [][2]float64 `json:"aggregate"`
	AggregateMax  [][2]float64 `json:"aggregate_max"`
	SeriesOmitted bool         `json:"series_omitted"`
	Series        []struct {
		Labels map[string]string `json:"labels"`
		Points [][2]float64      `json:"points"`
	} `json:"series"`
}

// LoadSeries reads and decodes a's referenced AIBOMTelemetry object. It does
// not check the digest -- call VerifySeries for that.
func LoadSeries(ctx context.Context, client dynamic.Interface, a AIBOM) (Series, error) {
	var s Series
	ref, ok := a.RawData["telemetry_series_ref"].(map[string]any)
	if !ok {
		return s, fmt.Errorf("aibom %s has no stored telemetry series (telemetry_series_ref)", a.Name)
	}
	name, _ := ref["name"].(string)
	if ref["kind"] != telemetryKind || name == "" {
		return s, fmt.Errorf("aibom %s has a telemetry_series_ref this version can't read", a.Name)
	}
	obj, err := client.Resource(TelemetryGVR).Namespace(a.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return s, fmt.Errorf("getting %s/%s: %w", telemetryKind, name, err)
	}
	raw, found, err := unstructured.NestedString(obj.Object, "spec", "seriesJson")
	if err != nil || !found {
		return s, fmt.Errorf("%s/%s has no spec.seriesJson string", telemetryKind, name)
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return s, fmt.Errorf("decoding %s/%s: %w", telemetryKind, name, err)
	}
	return s, nil
}
