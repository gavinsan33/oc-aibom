package aibom

import (
	"fmt"
	"sort"
	"strings"
)

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
	{"Optimizer", func(a AIBOM) string {
		if a.Data.Training == nil {
			return ""
		}
		return a.Data.Training.Optimizer
	}},
	{"LR Scheduler", func(a AIBOM) string {
		if a.Data.Training == nil || a.Data.Training.LRScheduler == nil {
			return ""
		}
		return a.Data.Training.LRScheduler.Type
	}},
	{"Mixed Precision", func(a AIBOM) string { return training(a).MixedPrecision }},
	{"Parallelism", func(a AIBOM) string { return training(a).Parallelism() }},
	{"Topology", func(a AIBOM) string { return training(a).Distributed.Topology() }},
	{"Parallel Sizes", func(a AIBOM) string { return training(a).Distributed.ParallelSizes() }},
	{"FSDP", func(a AIBOM) string { return training(a).Distributed.FSDPSummary() }},
	{"DeepSpeed", func(a AIBOM) string { return training(a).Distributed.DeepSpeedSummary() }},
	{"GPU Type", func(a AIBOM) string { return a.Data.Environment.GPUType }},
	{"GPU Count", func(a AIBOM) string { return fmt.Sprintf("%d", a.Data.Environment.GPUCount) }},
	{"CUDA Version", func(a AIBOM) string { return a.Data.Environment.CUDAVersion }},
	{"Driver Version", func(a AIBOM) string { return a.Data.Environment.DriverVersion }},
	{"Framework Version", func(a AIBOM) string { return a.Data.Environment.FrameworkVersion }},
	{"Python Version", func(a AIBOM) string { return a.Data.Environment.PythonVersion }},
	{"Torch CUDA Version", func(a AIBOM) string { return a.Data.Environment.TorchCUDAVersion }},
	{"Libraries", func(a AIBOM) string { return a.Data.Environment.LibraryList() }},
	{"AIBOM Version", func(a AIBOM) string { return a.Data.Metadata.AIBOMVersion }},
}

// training is the AIBOM's training block, or an empty one for an AIBOM
// without it (inference runs), so accessors need no nil check.
func training(a AIBOM) Training {
	if a.Data.Training == nil {
		return Training{}
	}
	return *a.Data.Training
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
		if fa.label == "Libraries" && differs {
			vals = libraryDifferences(items)
		}
		rows = append(rows, FieldRow{Label: fa.label, Values: vals, Differs: differs})
	}
	return rows
}

// libraryDifferences narrows each run's Libraries value to the libraries
// whose version isn't the same in every run ("not imported" where a run
// didn't import it), plus how many others match, so a one-library change isn't a whole
// differing list. Runs that recorded no libraries stay empty and don't count.
func libraryDifferences(items []AIBOM) []string {
	recorded := []map[string]string{}
	for _, a := range items {
		if len(a.Data.Environment.Libraries) > 0 {
			recorded = append(recorded, a.Data.Environment.Libraries)
		}
	}
	names := map[string]bool{}
	for _, libs := range recorded {
		for n := range libs {
			names[n] = true
		}
	}
	differing, same := []string{}, 0
	for n := range names {
		v, ok := recorded[0][n]
		alike := true
		for _, libs := range recorded[1:] {
			if w, has := libs[n]; has != ok || w != v {
				alike = false
				break
			}
		}
		if alike {
			same++
		} else {
			differing = append(differing, n)
		}
	}
	sort.Strings(differing)
	vals := make([]string, len(items))
	for i, a := range items {
		libs := a.Data.Environment.Libraries
		if len(libs) == 0 {
			continue
		}
		parts := make([]string, len(differing))
		for j, n := range differing {
			v, ok := libs[n]
			if !ok {
				v = "not imported"
			}
			parts[j] = n + " " + v
		}
		vals[i] = strings.Join(parts, ", ")
		if same > 0 {
			vals[i] += fmt.Sprintf(" (+%d identical)", same)
		}
	}
	return vals
}

// MetricRow is one resource_utilization metric's average compared across N
// AIBOMs. Available[i] is false when run i has no value for the metric --
// resource_utilization.note is set, or the metric was never collected -- so its
// avg is a meaningless 0, shown as "-". Delta and PctChange are only
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
		m, ok := ru.Metrics[k]
		return ru.MetricAvg(k), ru.Note == "" && ok, m.Unit
	})
}

// RuntimeKey is CompareRuntime's MetricRow key.
const RuntimeKey = "runtime"

// CompareRuntime is a MetricRow of each run's wall-clock duration in seconds
// (execution_metadata.duration_seconds), which leads the Hardware Performance
// table so how long each run took sits next to what it used -- as the console
// plugin's Runtime row. Unavailable only when the duration wasn't recorded; a
// resource_utilization note doesn't blank it.
func CompareRuntime(items []AIBOM) MetricRow {
	return comparePerformance(items, []string{RuntimeKey}, func(a AIBOM, _ string) (float64, bool, string) {
		d := a.Data.ExecutionMetadata.DurationSeconds
		if d == nil {
			return 0, false, "s"
		}
		return *d, true, "s"
	})[0]
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
		// nil = not reconciled (declared inferred from runtime), not drift.
		if d.MatchesDeclared != nil && !*d.MatchesDeclared {
			return true
		}
	}
	return false
}
