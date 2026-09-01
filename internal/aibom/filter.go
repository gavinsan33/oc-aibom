package aibom

import "strings"

// Filter narrows a list of AIBOMs by exact-match (case-insensitive) on the
// given criteria. Empty fields are not filtered on.
type Filter struct {
	Model            string
	Intent           string
	Quantization     string
	Architecture     string
	Framework        string
	GPUType          string
	JobName          string
	GitBranch        string
	GitRepository    string
	ServingEngine    string
	AdaptationMethod string
	Optimizer        string
}

func (f Filter) Match(a AIBOM) bool {
	if f.Model != "" && !strings.EqualFold(a.Data.Model.Name, f.Model) {
		return false
	}
	if f.Intent != "" && !strings.EqualFold(a.ExperimentIntent, f.Intent) {
		return false
	}
	if f.Quantization != "" && !strings.EqualFold(a.Data.Model.Quantization, f.Quantization) {
		return false
	}
	if f.Architecture != "" && !strings.EqualFold(a.Data.Model.Architecture, f.Architecture) {
		return false
	}
	if f.Framework != "" && !strings.EqualFold(a.Data.Model.Framework, f.Framework) {
		return false
	}
	if f.GPUType != "" && !strings.EqualFold(a.Data.Environment.GPUType, f.GPUType) {
		return false
	}
	if f.JobName != "" && !strings.EqualFold(a.JobName, f.JobName) {
		return false
	}
	if f.GitBranch != "" && !strings.EqualFold(a.Data.SourceCode.GitBranch, f.GitBranch) {
		return false
	}
	if f.GitRepository != "" && !strings.EqualFold(a.Data.SourceCode.GitRepository, f.GitRepository) {
		return false
	}
	if f.ServingEngine != "" && (a.Data.Inference == nil || !strings.EqualFold(a.Data.Inference.ServingEngine, f.ServingEngine)) {
		return false
	}
	if f.AdaptationMethod != "" && (a.Data.FineTuning == nil || !strings.EqualFold(a.Data.FineTuning.AdaptationMethod, f.AdaptationMethod)) {
		return false
	}
	if f.Optimizer != "" && (a.Data.Training == nil || !strings.EqualFold(a.Data.Training.Optimizer, f.Optimizer)) {
		return false
	}
	return true
}

// Apply returns the subset of items matching f.
func Apply(items []AIBOM, f Filter) []AIBOM {
	var out []AIBOM
	for _, a := range items {
		if f.Match(a) {
			out = append(out, a)
		}
	}
	return out
}

// DriftOnly returns AIBOMs whose auto-detected dataset(s) disagree with the
// declared dataset, surfacing reconciliation drift without requiring the
// caller to inspect spec.data.dataset manually.
func DriftOnly(items []AIBOM) []AIBOM {
	var out []AIBOM
	for _, a := range items {
		for _, d := range a.Data.Dataset.AutoDetected {
			if !d.MatchesDeclared {
				out = append(out, a)
				break
			}
		}
	}
	return out
}
