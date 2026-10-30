package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

// trainingExtras returns the optional Training lines (label, value) for
// `describe`: optimizer settings, LR scheduler, mixed precision and
// determinism. Anything absent is skipped, so an older AIBOM prints nothing
// extra.
func trainingExtras(t aibom.Training) [][2]string {
	var out [][2]string
	add := func(label, v string) {
		if v != "" {
			out = append(out, [2]string{label, v})
		}
	}

	if oc := t.OptimizerConfig; oc != nil {
		add("Optimizer Class", withVia(oc.Class, oc.DeclaredVia))
		params := formatSettings(oc.Params)
		if oc.Class == "" {
			// A declared config has no class to hang `via` on.
			params = withVia(params, oc.DeclaredVia)
		}
		add("Optimizer Params", params)
		groups := make([]string, 0, len(oc.ParamGroups))
		for _, g := range oc.ParamGroups {
			settings := make(map[string]any, len(g))
			for k, v := range g {
				if k != "param_tensors" {
					settings[k] = v
				}
			}
			group := formatSettings(settings)
			if n, ok := g["param_tensors"]; ok {
				group += fmt.Sprintf(" (%s tensors)", formatValue(n))
			}
			groups = append(groups, group)
		}
		add("Param Groups", strings.Join(groups, "; "))
		add("Optim Args", oc.OptimArgs)
	}
	if s := t.LRScheduler; s != nil {
		parts := []string{}
		if s.Type != "" {
			parts = append(parts, s.Type)
		}
		if s.WarmupRatio != nil {
			parts = append(parts, "warmup ratio "+formatValue(*s.WarmupRatio))
		}
		if s.WarmupSteps != nil {
			parts = append(parts, "warmup steps "+formatValue(*s.WarmupSteps))
		}
		add("LR Scheduler", withVia(strings.Join(parts, ", "), s.DeclaredVia))
	}
	add("Mixed Precision", t.MixedPrecision)

	if d := t.Determinism; d != nil {
		add("Seeds", formatSeeds(d.Seeds))
		history := []string{}
		for _, g := range seedGeneratorOrder(d.SeedHistory) {
			values := make([]string, len(d.SeedHistory[g]))
			for i, v := range d.SeedHistory[g] {
				values[i] = formatValue(v)
			}
			history = append(history, g+": "+strings.Join(values, " → "))
		}
		add("Seed History", strings.Join(history, "; "))
		if d.DataSeed != nil {
			add("Data Seed", formatValue(d.DataSeed))
		}
		flags := []string{}
		flag := func(label string, v *bool) {
			if v != nil {
				flags = append(flags, fmt.Sprintf("%s %v", label, *v))
			}
		}
		flag("cudnn deterministic", d.CudnnDeterministic)
		flag("cudnn benchmark", d.CudnnBenchmark)
		if d.DeterministicAlgorithms != nil {
			algorithms := fmt.Sprintf("deterministic algorithms %v", *d.DeterministicAlgorithms)
			if d.DeterministicAlgorithmsWarnOnly {
				algorithms += " (warn only)"
			}
			flags = append(flags, algorithms)
		}
		if d.FullDeterminism {
			flags = append(flags, "full determinism")
		}
		add("Determinism", strings.Join(flags, ", "))
		add("CUBLAS Workspace", d.CublasWorkspaceConfig)
		add("PYTHONHASHSEED", d.PythonHashSeed)
	}
	return out
}

func withVia(v, via string) string {
	if v == "" || via == "" {
		return v
	}
	return fmt.Sprintf("%s (via: %s)", v, via)
}

// formatSettings renders a settings map as "k=v, ..." in key order.
func formatSettings(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + formatValue(m[k])
	}
	return strings.Join(parts, ", ")
}

// formatSeeds renders seeds as "python=42, numpy=42, torch=42": the three
// generators the webhook records first, in that order, then anything else.
func formatSeeds(m map[string]any) string {
	parts := []string{}
	for _, g := range seedGeneratorOrder(m) {
		parts = append(parts, g+"="+formatValue(m[g]))
	}
	return strings.Join(parts, ", ")
}

func seedGeneratorOrder[V any](m map[string]V) []string {
	order := []string{}
	for _, g := range []string{"python", "numpy", "torch"} {
		if _, ok := m[g]; ok {
			order = append(order, g)
		}
	}
	rest := []string{}
	for g := range m {
		if g != "python" && g != "numpy" && g != "torch" {
			rest = append(rest, g)
		}
	}
	sort.Strings(rest)
	return append(order, rest...)
}

// formatValue renders a decoded JSON value: floats in their shortest form
// (0.0002, 42, 1e-08), lists as "[a, b]".
func formatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case []any:
		items := make([]string, len(x))
		for i, item := range x {
			items[i] = formatValue(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	default:
		return fmt.Sprint(x)
	}
}
