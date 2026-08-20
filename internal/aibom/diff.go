package aibom

import (
	"fmt"
	"math"
)

// FieldDiff is a single field that differs between two AIBOMs.
type FieldDiff struct {
	Field string
	A     string
	B     string
}

// MetricDiff is a quantified comparison of one resource_utilization metric
// between two AIBOMs. PctChange is relative to A; it is NaN when A is zero
// (division by zero), which callers should render as "N/A".
type MetricDiff struct {
	Metric    string
	A         float64
	B         float64
	Delta     float64 // B - A
	PctChange float64 // (B - A) / A * 100
}

// metricSpec names a resource_utilization field and how to extract it, so
// DiffPerformance and the compare table can share one definition of "which
// metrics matter" instead of drifting apart.
type metricSpec struct {
	Name string
	Get  func(ResourceUtilization) float64
}

var performanceMetrics = []metricSpec{
	{"avg_gpu_utilization_pct", func(r ResourceUtilization) float64 { return r.AvgGPUUtilizationPct }},
	{"avg_gpu_memory_used_mib", func(r ResourceUtilization) float64 { return r.AvgGPUMemoryUsedMiB }},
	{"avg_gpu_power_watts", func(r ResourceUtilization) float64 { return r.AvgGPUPowerWatts }},
	{"avg_cpu_usage_cores", func(r ResourceUtilization) float64 { return r.AvgCPUUsageCores }},
	{"avg_memory_usage_gb", func(r ResourceUtilization) float64 { return r.AvgMemoryUsageGB }},
	{"avg_network_receive_mbps", func(r ResourceUtilization) float64 { return r.AvgNetworkReceiveMbps }},
	{"avg_network_transmit_mbps", func(r ResourceUtilization) float64 { return r.AvgNetworkTransmitMbps }},
}

// DiffPerformance quantifies the change in each resource_utilization metric
// from a to b. Unlike Diff, it always returns one entry per metric (even
// when unchanged), since "no change" is itself a useful performance-compare
// result, not something to omit.
func DiffPerformance(a, b AIBOM) []MetricDiff {
	diffs := make([]MetricDiff, 0, len(performanceMetrics))
	for _, m := range performanceMetrics {
		av := m.Get(a.Data.ResourceUtilization)
		bv := m.Get(b.Data.ResourceUtilization)
		pct := math.NaN()
		if av != 0 {
			pct = (bv - av) / av * 100
		}
		diffs = append(diffs, MetricDiff{
			Metric:    m.Name,
			A:         av,
			B:         bv,
			Delta:     bv - av,
			PctChange: pct,
		})
	}
	return diffs
}

// Diff compares the fields most useful for spotting drift between two runs:
// model config, dataset declaration/reconciliation, source provenance,
// hardware/driver environment, and schema version. It does not attempt a
// generic deep-diff of the whole spec.data document.
func Diff(a, b AIBOM) []FieldDiff {
	var diffs []FieldDiff
	add := func(field, av, bv string) {
		if av != bv {
			diffs = append(diffs, FieldDiff{Field: field, A: av, B: bv})
		}
	}

	add("jobName", a.JobName, b.JobName)
	add("experimentIntent", a.ExperimentIntent, b.ExperimentIntent)

	add("model.name", a.Data.Model.Name, b.Data.Model.Name)
	add("model.version", a.Data.Model.Version, b.Data.Model.Version)
	add("model.architecture", a.Data.Model.Architecture, b.Data.Model.Architecture)
	add("model.framework", a.Data.Model.Framework, b.Data.Model.Framework)
	add("model.quantization", a.Data.Model.Quantization, b.Data.Model.Quantization)
	add("model.dtype", a.Data.Model.Dtype, b.Data.Model.Dtype)

	add("dataset.declared.name", a.Data.Dataset.Declared.Name, b.Data.Dataset.Declared.Name)
	add("dataset.declared.version", a.Data.Dataset.Declared.Version, b.Data.Dataset.Declared.Version)
	add("dataset.declared.license", a.Data.Dataset.Declared.License, b.Data.Dataset.Declared.License)
	add("dataset.drift", fmt.Sprintf("%v", hasDrift(a)), fmt.Sprintf("%v", hasDrift(b)))

	add("source_code.git_repository", a.Data.SourceCode.GitRepository, b.Data.SourceCode.GitRepository)
	add("source_code.git_commit", a.Data.SourceCode.GitCommit, b.Data.SourceCode.GitCommit)
	add("source_code.git_branch", a.Data.SourceCode.GitBranch, b.Data.SourceCode.GitBranch)
	add("source_code.dirty", fmt.Sprintf("%v", a.Data.SourceCode.Dirty), fmt.Sprintf("%v", b.Data.SourceCode.Dirty))

	add("environment.gpu_type", a.Data.Environment.GPUType, b.Data.Environment.GPUType)
	add("environment.gpu_count", fmt.Sprintf("%d", a.Data.Environment.GPUCount), fmt.Sprintf("%d", b.Data.Environment.GPUCount))
	add("environment.cuda_version", a.Data.Environment.CUDAVersion, b.Data.Environment.CUDAVersion)
	add("environment.driver_version", a.Data.Environment.DriverVersion, b.Data.Environment.DriverVersion)
	add("environment.framework_version", a.Data.Environment.FrameworkVersion, b.Data.Environment.FrameworkVersion)

	add("_metadata.aibom_version", a.Data.Metadata.AIBOMVersion, b.Data.Metadata.AIBOMVersion)

	return diffs
}

func hasDrift(a AIBOM) bool {
	for _, d := range a.Data.Dataset.AutoDetected {
		if !d.MatchesDeclared {
			return true
		}
	}
	return false
}
