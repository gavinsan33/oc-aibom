package aibom

import "fmt"

// FieldRow is one metadata field compared across N AIBOMs, in the fixed order
// of compareFieldAccessors. Differs is set when the AIBOMs don't all share the
// same value, so a renderer can highlight (or, with --changed, filter to) the
// fields that actually vary. This is the N-way generalization of the old
// pairwise field diff; `diff` is just `compare --changed` over two AIBOMs.
type FieldRow struct {
	Label   string
	Values  []string
	Differs bool
}

type fieldAccessor struct {
	label string
	get   func(AIBOM) string
}

// compareFieldAccessors is the field set and order compared across runs,
// mirroring the console plugin's FIELD_ACCESSORS so the CLI and console show
// the same rows.
var compareFieldAccessors = []fieldAccessor{
	{"Job", func(a AIBOM) string { return a.JobName }},
	{"Experiment Intent", func(a AIBOM) string { return a.ExperimentIntent }},
	{"Model Name", func(a AIBOM) string { return a.Data.Model.Name }},
	{"Model Version", func(a AIBOM) string { return a.Data.Model.Version }},
	{"Model Revision", func(a AIBOM) string { return a.Data.Model.Revision }},
	{"Architecture", func(a AIBOM) string { return a.Data.Model.Architecture }},
	{"Framework", func(a AIBOM) string { return a.Data.Model.Framework }},
	{"Quantization", func(a AIBOM) string { return a.Data.Model.Quantization }},
	{"Dtype", func(a AIBOM) string { return a.Data.Model.Dtype }},
	{"Declared Dataset", func(a AIBOM) string { return a.Data.Dataset.Declared.Name }},
	{"Declared Dataset Version", func(a AIBOM) string { return a.Data.Dataset.Declared.Version }},
	{"Declared Dataset License", func(a AIBOM) string { return a.Data.Dataset.Declared.License }},
	{"Dataset Drift", func(a AIBOM) string { return fmt.Sprintf("%v", hasDrift(a)) }},
	{"Git Repository", func(a AIBOM) string { return a.Data.SourceCode.GitRepository }},
	{"Git Commit", func(a AIBOM) string { return a.Data.SourceCode.GitCommit }},
	{"Git Branch", func(a AIBOM) string { return a.Data.SourceCode.GitBranch }},
	{"Dirty", func(a AIBOM) string { return fmt.Sprintf("%v", a.Data.SourceCode.Dirty) }},
	{"GPU Type", func(a AIBOM) string { return a.Data.Environment.GPUType }},
	{"GPU Count", func(a AIBOM) string { return fmt.Sprintf("%d", a.Data.Environment.GPUCount) }},
	{"CUDA Version", func(a AIBOM) string { return a.Data.Environment.CUDAVersion }},
	{"Driver Version", func(a AIBOM) string { return a.Data.Environment.DriverVersion }},
	{"Framework Version", func(a AIBOM) string { return a.Data.Environment.FrameworkVersion }},
	{"AIBOM Version", func(a AIBOM) string { return a.Data.Metadata.AIBOMVersion }},
}

// CompareFields returns one FieldRow per compared field, with each AIBOM's
// value and whether they all agree. Every field is always returned; callers
// decide whether to hide the unchanged ones.
func CompareFields(items []AIBOM) []FieldRow {
	rows := make([]FieldRow, 0, len(compareFieldAccessors))
	for _, fa := range compareFieldAccessors {
		vals := make([]string, len(items))
		differs := false
		for i, a := range items {
			vals[i] = fa.get(a)
			if i > 0 && vals[i] != vals[0] {
				differs = true
			}
		}
		rows = append(rows, FieldRow{Label: fa.label, Values: vals, Differs: differs})
	}
	return rows
}

// MetricRow is one resource_utilization metric's average compared across N
// AIBOMs. Available[i] is false when run i's resource_utilization.note is set
// (its avg is meaningless, shown as "—"). Delta and PctChange are only
// populated for a two-run comparison (B relative to A); PctChange is nil when
// the baseline avg is zero.
type MetricRow struct {
	Key       string
	Unit      string
	Values    []float64
	Available []bool
	Delta     *float64
	PctChange *float64
}

// ComparePerformance builds one MetricRow per key (keys given in display
// order), averaging each hardware (resource_utilization) metric across the
// runs. For exactly two runs it also quantifies the change, matching the
// console plugin's pairwise Delta/Change columns.
func ComparePerformance(items []AIBOM, keys []string) []MetricRow {
	return comparePerformance(items, keys, func(a AIBOM, k string) (float64, bool, string) {
		ru := a.Data.ResourceUtilization
		return ru.MetricAvg(k), ru.Note == "", ru.Metrics[k].Unit
	})
}

// CompareInferencePerformance is ComparePerformance over inference.performance
// (serving-engine) metrics instead of hardware ones. Availability is per
// metric, not per run: engines export different subsets (TGI has no TTFT,
// KV-cache or prompt-throughput metric), so a run is "available" for a key
// only when that key is present. An AIBOM with no inference block shows "—"
// for every inference metric.
func CompareInferencePerformance(items []AIBOM, keys []string) []MetricRow {
	return comparePerformance(items, keys, func(a AIBOM, k string) (float64, bool, string) {
		if a.Data.Inference == nil || a.Data.Inference.Performance == nil {
			return 0, false, ""
		}
		m, ok := a.Data.Inference.Performance.Metrics[k]
		return m.Avg, ok, m.Unit
	})
}

func comparePerformance(items []AIBOM, keys []string, metric func(AIBOM, string) (avg float64, available bool, unit string)) []MetricRow {
	rows := make([]MetricRow, 0, len(keys))
	for _, k := range keys {
		r := MetricRow{Key: k, Values: make([]float64, len(items)), Available: make([]bool, len(items))}
		for i, a := range items {
			var unit string
			r.Values[i], r.Available[i], unit = metric(a, k)
			if r.Unit == "" {
				r.Unit = unit
			}
		}
		// No delta against a run without data: its 0 is a placeholder.
		if len(items) == 2 && r.Available[0] && r.Available[1] {
			delta := r.Values[1] - r.Values[0]
			r.Delta = &delta
			if r.Values[0] != 0 {
				pct := delta / r.Values[0] * 100
				r.PctChange = &pct
			}
		}
		rows = append(rows, r)
	}
	return rows
}

func hasDrift(a AIBOM) bool {
	for _, d := range a.Data.Dataset.AutoDetected {
		if !d.MatchesDeclared {
			return true
		}
	}
	return false
}
