package aibom

import "fmt"

// FieldDiff is a single field that differs between two AIBOMs.
type FieldDiff struct {
	Field string
	A     string
	B     string
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
