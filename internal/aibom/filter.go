package aibom

import "strings"

// Filter narrows a list of AIBOMs by exact-match (case-insensitive) on the
// given criteria. Empty fields are not filtered on.
type Filter struct {
	Model        string
	Intent       string
	Quantization string
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
