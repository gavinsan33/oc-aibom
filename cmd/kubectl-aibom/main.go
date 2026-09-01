// Command kubectl-aibom is a kubectl/oc plugin (invoked as `kubectl aibom`
// or `oc aibom`) that makes it easier to list, inspect, and compare AIBOM
// custom resources (group aibom.io) than raw `oc get aibom -o yaml`.
package main

import (
	"context"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func main() {
	configFlags := genericclioptions.NewConfigFlags(true)

	var noColor bool
	root := &cobra.Command{
		Use:   "aibom",
		Short: "List, inspect, and compare AIBOM custom resources",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initColor(noColor)
		},
	}
	configFlags.AddFlags(root.PersistentFlags())
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	var allNamespaces bool
	var modelFilter, intentFilter, quantFilter string
	var architectureFilter, frameworkFilter, gpuTypeFilter string
	var jobFilter, gitBranchFilter, gitRepoFilter string
	var servingEngineFilter, adaptationMethodFilter, optimizerFilter string
	var driftOnly bool
	var sortBy string
	var ascending bool
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "get"},
		Short:   "List AIBOMs, optionally filtered by model/intent/quantization",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ns := namespace
			if allNamespaces {
				ns = ""
			}
			items, err := aibom.List(context.Background(), client, ns)
			if err != nil {
				return err
			}
			items = aibom.Apply(items, aibom.Filter{
				Model:            modelFilter,
				Intent:           intentFilter,
				Quantization:     quantFilter,
				Architecture:     architectureFilter,
				Framework:        frameworkFilter,
				GPUType:          gpuTypeFilter,
				JobName:          jobFilter,
				GitBranch:        gitBranchFilter,
				GitRepository:    gitRepoFilter,
				ServingEngine:    servingEngineFilter,
				AdaptationMethod: adaptationMethodFilter,
				Optimizer:        optimizerFilter,
			})
			if driftOnly {
				items = aibom.DriftOnly(items)
			}
			if sortBy != "" {
				if err := aibom.SortByMetric(items, sortBy, ascending); err != nil {
					return err
				}
			}
			printList(items, allNamespaces, sortBy)
			return nil
		},
	}
	listCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "list AIBOMs across all namespaces")
	listCmd.Flags().StringVar(&modelFilter, "model", "", "filter by model.name")
	listCmd.Flags().StringVar(&intentFilter, "intent", "", "filter by experiment intent (training|sft|inference)")
	listCmd.Flags().StringVar(&quantFilter, "quantization", "", "filter by model.quantization")
	listCmd.Flags().StringVar(&architectureFilter, "architecture", "", "filter by model.architecture")
	listCmd.Flags().StringVar(&frameworkFilter, "framework", "", "filter by model.framework")
	listCmd.Flags().StringVar(&gpuTypeFilter, "gpu-type", "", "filter by environment.gpu_type")
	listCmd.Flags().StringVar(&jobFilter, "job", "", "filter by job name")
	listCmd.Flags().StringVar(&gitBranchFilter, "git-branch", "", "filter by source_code.git_branch")
	listCmd.Flags().StringVar(&gitRepoFilter, "git-repository", "", "filter by source_code.git_repository")
	listCmd.Flags().StringVar(&servingEngineFilter, "serving-engine", "", "filter by inference.serving_engine")
	listCmd.Flags().StringVar(&adaptationMethodFilter, "adaptation-method", "", "filter by fine_tuning.adaptation_method")
	listCmd.Flags().StringVar(&optimizerFilter, "optimizer", "", "filter by training.optimizer")
	listCmd.Flags().BoolVar(&driftOnly, "drift-only", false, "only show AIBOMs where auto-detected dataset(s) disagree with the declared dataset")
	listCmd.Flags().StringVar(&sortBy, "sort-by", "", "rank by a performance metric: gpu-utilization, gpu-memory, gpu-power, cpu-usage, memory-usage, network-rx, network-tx (highest first)")
	listCmd.Flags().BoolVar(&ascending, "ascending", false, "reverse --sort-by order (lowest first)")

	getCmd := &cobra.Command{
		Use:               "describe <name>",
		Short:             "Print a human-readable summary of a single AIBOM",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAIBOMNames(configFlags, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			a, err := aibom.Get(context.Background(), client, namespace, args[0])
			if err != nil {
				return err
			}
			printDescribe(a)
			return nil
		},
	}

	diffCmd := &cobra.Command{
		Use:               "diff <name-a> <name-b>",
		Short:             "Show field-level differences between two AIBOMs",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAIBOMNames(configFlags, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ctx := context.Background()
			a, err := aibom.Get(ctx, client, namespace, args[0])
			if err != nil {
				return err
			}
			b, err := aibom.Get(ctx, client, namespace, args[1])
			if err != nil {
				return err
			}
			printDiff(args[0], args[1], aibom.Diff(a, b), aibom.DiffPerformance(a, b))
			return nil
		},
	}

	compareCmd := &cobra.Command{
		Use:               "compare <name> <name> [<name>...]",
		Short:             "Show performance metrics for two or more AIBOMs side by side",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeAIBOMNames(configFlags, 0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ctx := context.Background()
			items := make([]aibom.AIBOM, 0, len(args))
			for _, name := range args {
				a, err := aibom.Get(ctx, client, namespace, name)
				if err != nil {
					return err
				}
				items = append(items, a)
			}
			printCompare(items)
			return nil
		},
	}

	root.AddCommand(listCmd, getCmd, diffCmd, compareCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// completeAIBOMNames returns a cobra ValidArgsFunction that suggests AIBOM
// names from the target namespace (respecting -n/--namespace), excluding
// names already given as positional args. maxArgs caps how many positional
// args this command accepts (0 means unlimited, e.g. `compare`); once
// reached, no further names are suggested.
func completeAIBOMNames(configFlags *genericclioptions.ConfigFlags, maxArgs int) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if maxArgs > 0 && len(args) >= maxArgs {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, namespace, err := buildClient(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		items, err := aibom.List(context.Background(), client, namespace)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		given := make(map[string]bool, len(args))
		for _, a := range args {
			given[a] = true
		}
		var names []string
		for _, item := range items {
			if !given[item.Name] {
				names = append(names, item.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func buildClient(configFlags *genericclioptions.ConfigFlags) (dynamic.Interface, string, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, "", fmt.Errorf("building kube config: %w", err)
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("building dynamic client: %w", err)
	}
	namespace, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("resolving namespace: %w", err)
	}
	return client, namespace, nil
}

// metricLabels are the display headers for aibom.SortableMetrics keys.
var metricLabels = map[string]string{
	"gpu-utilization": "GPU UTIL %",
	"gpu-memory":      "GPU MEM MIB",
	"gpu-power":       "GPU POWER W",
	"cpu-usage":       "CPU CORES",
	"memory-usage":    "MEM GB",
	"network-rx":      "NET RX MBPS",
	"network-tx":      "NET TX MBPS",
}

// telemetryMetricOrder and telemetryMetricLabels give a fixed display order
// and label for ResourceUtilization.Metrics, which is keyed by the same raw
// names as aibom-webhook-service's TELEMETRY_QUERIES (a Go map has no
// inherent order).
var telemetryMetricOrder = []string{
	"gpu_utilization", "gpu_memory_used", "gpu_power",
	"cpu_usage", "memory_usage", "network_receive", "network_transmit",
}

var telemetryMetricLabels = map[string]string{
	"gpu_utilization":  "GPU Utilization",
	"gpu_memory_used":  "GPU Memory",
	"gpu_power":        "GPU Power",
	"cpu_usage":        "CPU Usage",
	"memory_usage":     "Memory Usage",
	"network_receive":  "Network RX",
	"network_transmit": "Network TX",
}

// trendArrowParts returns a trend arrow as both plain text (visible, for
// column-width calculation) and colorized text (rendered, for display) --
// see the cell type's doc comment in color.go for why these must differ.
func trendArrowParts(trend string) (visible, rendered string) {
	switch trend {
	case "up":
		return "↑", yellow("↑")
	case "down":
		return "↓", yellow("↓")
	case "flat":
		return "→", "→"
	default:
		return "", ""
	}
}

// trendArrow is trendArrowParts' rendered form, for plain (non-table) output
// like printMetricDetail where there's no column alignment to protect.
func trendArrow(trend string) string {
	_, rendered := trendArrowParts(trend)
	return rendered
}

func formatSegment(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *v)
}

func printList(items []aibom.AIBOM, allNamespaces bool, sortBy string) {
	metricHeader, metricGet := "", aibom.SortableMetrics[sortBy]
	if sortBy != "" {
		metricHeader = metricLabels[sortBy]
	}

	cols := []string{"NAME", "JOB", "MODEL", "INTENT", "QUANTIZATION", "GPU TYPE", "COLLECTED AT"}
	if allNamespaces {
		cols = append([]string{"NAMESPACE"}, cols...)
	}
	if metricHeader != "" {
		cols = append(cols, metricHeader)
	}
	rows := [][]cell{headerRow(cols...)}

	for _, a := range items {
		values := []string{
			a.Name, a.JobName, a.Data.Model.Name, a.ExperimentIntent,
			a.Data.Model.Quantization, a.Data.Environment.GPUType, a.CollectedAt,
		}
		if allNamespaces {
			values = append([]string{a.Namespace}, values...)
		}
		if metricGet != nil {
			values = append(values, formatMetric(metricGet(a.Data.ResourceUtilization)))
		}
		rows = append(rows, plainRow(values...))
	}
	writeTable(os.Stdout, rows)
	if len(items) == 0 {
		fmt.Println("No AIBOMs found.")
	}
}

func formatMetric(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func formatPctChange(v float64) string {
	if math.IsNaN(v) {
		return "N/A"
	}
	return fmt.Sprintf("%+.1f%%", v)
}

func printDescribe(a aibom.AIBOM) {
	fmt.Printf("Name:              %s\n", a.Name)
	fmt.Printf("Namespace:         %s\n", a.Namespace)
	fmt.Printf("Job:               %s\n", a.JobName)
	fmt.Printf("Experiment Intent: %s\n", a.ExperimentIntent)
	fmt.Printf("Collected At:      %s\n", a.CollectedAt)
	fmt.Println()
	fmt.Println(bold("Model:"))
	fmt.Printf("  Name:          %s\n", a.Data.Model.Name)
	fmt.Printf("  Version:       %s\n", a.Data.Model.Version)
	fmt.Printf("  Architecture:  %s\n", a.Data.Model.Architecture)
	fmt.Printf("  Framework:     %s\n", a.Data.Model.Framework)
	fmt.Printf("  Quantization:  %s (%d-bit)\n", a.Data.Model.Quantization, a.Data.Model.QuantizationBits)
	fmt.Println()
	fmt.Println(bold("Dataset:"))
	fmt.Printf("  Declared:      %s %s (license: %s)\n", a.Data.Dataset.Declared.Name, a.Data.Dataset.Declared.Version, a.Data.Dataset.Declared.License)
	for _, d := range a.Data.Dataset.AutoDetected {
		match := green("matches declared")
		if !d.MatchesDeclared {
			match = red("DOES NOT MATCH DECLARED")
		}
		fmt.Printf("  Auto-detected: %s %s (license: %s) — %s\n", d.DatasetName, d.Version, d.License, match)
	}
	fmt.Println()
	fmt.Println(bold("Source:"))
	fmt.Printf("  Repository:    %s\n", a.Data.SourceCode.GitRepository)
	dirty := fmt.Sprintf("%v", a.Data.SourceCode.Dirty)
	if a.Data.SourceCode.Dirty {
		dirty = yellow(dirty)
	}
	fmt.Printf("  Commit:        %s (branch: %s, dirty: %s)\n", a.Data.SourceCode.GitCommit, a.Data.SourceCode.GitBranch, dirty)
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Printf("  GPU:           %s x%d\n", a.Data.Environment.GPUType, a.Data.Environment.GPUCount)
	fmt.Printf("  CUDA/Driver:   %s / %s\n", a.Data.Environment.CUDAVersion, a.Data.Environment.DriverVersion)
	fmt.Printf("  Framework:     %s\n", a.Data.Environment.FrameworkVersion)
	fmt.Println()
	fmt.Println("Performance:")
	ru := a.Data.ResourceUtilization
	if ru.Note != "" {
		fmt.Printf("  %s\n", ru.Note)
	} else {
		fmt.Printf("  GPU Utilization: %.2f%%\n", ru.AvgGPUUtilizationPct)
		fmt.Printf("  GPU Memory Used: %.2f MiB\n", ru.AvgGPUMemoryUsedMiB)
		fmt.Printf("  GPU Power:       %.2f W\n", ru.AvgGPUPowerWatts)
		fmt.Printf("  CPU Usage:       %.2f cores\n", ru.AvgCPUUsageCores)
		fmt.Printf("  Memory Usage:    %.2f GB\n", ru.AvgMemoryUsageGB)
		fmt.Printf("  Network RX/TX:   %.2f / %.2f Mbps\n", ru.AvgNetworkReceiveMbps, ru.AvgNetworkTransmitMbps)
		if ru.SummaryIncludesColdStart {
			fmt.Println("  (includes cold start)")
		}
		for _, link := range ru.GrafanaLinks {
			fmt.Printf("  Grafana:         %s\n", link)
		}
		printMetricDetail(ru)
	}
}

// printMetricDetail prints the min/max/p95 and within-run shape for each
// metric in ru.Metrics -- detail a flat average can't show, e.g. whether GPU
// utilization held steady or throttled down partway through the run. Silent
// no-op if the AIBOM predates this field (an older postprocess.py).
func printMetricDetail(ru aibom.ResourceUtilization) {
	if len(ru.Metrics) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(bold("Performance Detail (min / avg / max / p95, first→mid→last third of run):"))
	for _, key := range telemetryMetricOrder {
		m, ok := ru.Metrics[key]
		if !ok {
			continue
		}
		fmt.Printf(
			"  %-16s %.2f / %.2f / %.2f / %.2f %-13s  %s → %s → %s %s\n",
			telemetryMetricLabels[key]+":", m.Min, m.Avg, m.Max, m.P95, m.Unit,
			formatSegment(m.Segments.FirstThird), formatSegment(m.Segments.MiddleThird), formatSegment(m.Segments.LastThird),
			trendArrow(m.Segments.Trend()),
		)
	}
}

func printDiff(nameA, nameB string, diffs []aibom.FieldDiff, metrics []aibom.MetricDiff) {
	if len(diffs) == 0 {
		fmt.Printf("No differences found between %s and %s (across compared config/metadata fields).\n", nameA, nameB)
	} else {
		rows := [][]cell{{labelHeaderCell("FIELD"), runHeaderCell(0, nameA), runHeaderCell(1, nameB)}}
		for _, d := range diffs {
			rows = append(rows, []cell{labelCell(d.Field), plainCell(d.A), plainCell(d.B)})
		}
		writeTable(os.Stdout, rows)
	}

	fmt.Println()
	fmt.Println(bold("Performance:"))
	rows := [][]cell{{
		labelHeaderCell("METRIC"), runHeaderCell(0, nameA), runHeaderCell(1, nameB),
		coloredCell("DELTA", bold("DELTA")), coloredCell("CHANGE", bold("CHANGE")),
	}}
	for _, m := range metrics {
		deltaText := fmt.Sprintf("%+.2f", m.Delta)
		changeText := formatPctChange(m.PctChange)
		rows = append(rows, []cell{
			labelCell(m.Metric),
			metricCellWithTrend(m.A, m.TrendA),
			metricCellWithTrend(m.B, m.TrendB),
			coloredCell(deltaText, colorBySign(m.Delta, deltaText)),
			coloredCell(changeText, colorBySign(m.PctChange, changeText)),
		})
	}
	writeTable(os.Stdout, rows)
}

// metricCellWithTrend appends a within-run trend arrow (see
// MetricSegments.Trend) to a metric value, when known -- e.g. "75.00 ↓"
// flags that a run's own average was still dropping as it progressed, a
// distinction the cross-run delta/change columns can't make on their own.
// Built as a cell (not a plain string) so the arrow's color escape codes
// don't get counted by writeTable's column-width calculation.
func metricCellWithTrend(v float64, trend string) cell {
	text := formatMetric(v)
	visibleArrow, renderedArrow := trendArrowParts(trend)
	if visibleArrow == "" {
		return plainCell(text)
	}
	return coloredCell(text+" "+visibleArrow, text+" "+renderedArrow)
}

func printCompare(items []aibom.AIBOM) {
	header := make([]cell, len(items)+1)
	header[0] = labelHeaderCell("FIELD")
	for i, a := range items {
		header[i+1] = runHeaderCell(i, a.Name)
	}
	rows := [][]cell{header}

	row := func(label string, values func(aibom.AIBOM) string) {
		cells := make([]cell, len(items)+1)
		cells[0] = labelCell(label)
		for i, a := range items {
			cells[i+1] = plainCell(values(a))
		}
		rows = append(rows, cells)
	}

	row("Model", func(a aibom.AIBOM) string { return a.Data.Model.Name })
	row("Quantization", func(a aibom.AIBOM) string { return a.Data.Model.Quantization })
	row("Intent", func(a aibom.AIBOM) string { return a.ExperimentIntent })
	row("GPU Type", func(a aibom.AIBOM) string { return a.Data.Environment.GPUType })
	writeTable(os.Stdout, rows)
	fmt.Println()

	rows = nil
	for _, m := range []struct {
		label string
		get   func(aibom.ResourceUtilization) float64
	}{
		{"GPU Utilization %", func(r aibom.ResourceUtilization) float64 { return r.AvgGPUUtilizationPct }},
		{"GPU Memory (MiB)", func(r aibom.ResourceUtilization) float64 { return r.AvgGPUMemoryUsedMiB }},
		{"GPU Power (W)", func(r aibom.ResourceUtilization) float64 { return r.AvgGPUPowerWatts }},
		{"CPU Usage (cores)", func(r aibom.ResourceUtilization) float64 { return r.AvgCPUUsageCores }},
		{"Memory Usage (GB)", func(r aibom.ResourceUtilization) float64 { return r.AvgMemoryUsageGB }},
		{"Network RX (Mbps)", func(r aibom.ResourceUtilization) float64 { return r.AvgNetworkReceiveMbps }},
		{"Network TX (Mbps)", func(r aibom.ResourceUtilization) float64 { return r.AvgNetworkTransmitMbps }},
	} {
		row(m.label, func(a aibom.AIBOM) string {
			if a.Data.ResourceUtilization.Note != "" {
				return "-"
			}
			return formatMetric(m.get(a.Data.ResourceUtilization))
		})
	}
	writeTable(os.Stdout, rows)
}
