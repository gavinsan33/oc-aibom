// Command kubectl-aibom is a kubectl/oc plugin (invoked as `kubectl aibom`
// or `oc aibom`) that makes it easier to list, inspect, and compare AIBOM
// custom resources (group aibom.io) than raw `oc get aibom -o yaml`.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func main() {
	configFlags := genericclioptions.NewConfigFlags(true)

	root := &cobra.Command{
		Use:   "aibom",
		Short: "List, inspect, and compare AIBOM custom resources",
	}
	configFlags.AddFlags(root.PersistentFlags())

	var allNamespaces bool
	var modelFilter, intentFilter, quantFilter string
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
				Model:        modelFilter,
				Intent:       intentFilter,
				Quantization: quantFilter,
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
	listCmd.Flags().BoolVar(&driftOnly, "drift-only", false, "only show AIBOMs where auto-detected dataset(s) disagree with the declared dataset")
	listCmd.Flags().StringVar(&sortBy, "sort-by", "", "rank by a performance metric: gpu-utilization, gpu-memory, gpu-power, cpu-usage, memory-usage, network-rx, network-tx (highest first)")
	listCmd.Flags().BoolVar(&ascending, "ascending", false, "reverse --sort-by order (lowest first)")

	getCmd := &cobra.Command{
		Use:   "describe <name>",
		Short: "Print a human-readable summary of a single AIBOM",
		Args:  cobra.ExactArgs(1),
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
		Use:   "diff <name-a> <name-b>",
		Short: "Show field-level differences between two AIBOMs",
		Args:  cobra.ExactArgs(2),
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
		Use:   "compare <name> <name> [<name>...]",
		Short: "Show performance metrics for two or more AIBOMs side by side",
		Args:  cobra.MinimumNArgs(2),
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

func printList(items []aibom.AIBOM, allNamespaces bool, sortBy string) {
	metricHeader, metricGet := "", aibom.SortableMetrics[sortBy]
	if sortBy != "" {
		metricHeader = metricLabels[sortBy]
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	header := "NAME\tJOB\tMODEL\tINTENT\tQUANTIZATION\tGPU TYPE\tCOLLECTED AT"
	if allNamespaces {
		header = "NAMESPACE\t" + header
	}
	if metricHeader != "" {
		header += "\t" + metricHeader
	}
	fmt.Fprintln(w, header)

	for _, a := range items {
		row := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
			a.Name, a.JobName, a.Data.Model.Name, a.ExperimentIntent,
			a.Data.Model.Quantization, a.Data.Environment.GPUType, a.CollectedAt)
		if allNamespaces {
			row = a.Namespace + "\t" + row
		}
		if metricGet != nil {
			row += "\t" + formatMetric(metricGet(a.Data.ResourceUtilization))
		}
		fmt.Fprintln(w, row)
	}
	w.Flush()
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
	fmt.Println("Model:")
	fmt.Printf("  Name:          %s\n", a.Data.Model.Name)
	fmt.Printf("  Version:       %s\n", a.Data.Model.Version)
	fmt.Printf("  Architecture:  %s\n", a.Data.Model.Architecture)
	fmt.Printf("  Framework:     %s\n", a.Data.Model.Framework)
	fmt.Printf("  Quantization:  %s (%d-bit)\n", a.Data.Model.Quantization, a.Data.Model.QuantizationBits)
	fmt.Println()
	fmt.Println("Dataset:")
	fmt.Printf("  Declared:      %s %s (license: %s)\n", a.Data.Dataset.Declared.Name, a.Data.Dataset.Declared.Version, a.Data.Dataset.Declared.License)
	for _, d := range a.Data.Dataset.AutoDetected {
		match := "matches declared"
		if !d.MatchesDeclared {
			match = "DOES NOT MATCH DECLARED"
		}
		fmt.Printf("  Auto-detected: %s %s (license: %s) — %s\n", d.DatasetName, d.Version, d.License, match)
	}
	fmt.Println()
	fmt.Println("Source:")
	fmt.Printf("  Repository:    %s\n", a.Data.SourceCode.GitRepository)
	fmt.Printf("  Commit:        %s (branch: %s, dirty: %v)\n", a.Data.SourceCode.GitCommit, a.Data.SourceCode.GitBranch, a.Data.SourceCode.Dirty)
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
	}
}

func printDiff(nameA, nameB string, diffs []aibom.FieldDiff, metrics []aibom.MetricDiff) {
	if len(diffs) == 0 {
		fmt.Printf("No differences found between %s and %s (across compared config/metadata fields).\n", nameA, nameB)
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "FIELD\t%s\t%s\n", nameA, nameB)
		for _, d := range diffs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", d.Field, d.A, d.B)
		}
		w.Flush()
	}

	fmt.Println()
	fmt.Println("Performance:")
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "METRIC\t%s\t%s\tDELTA\tCHANGE\n", nameA, nameB)
	for _, m := range metrics {
		fmt.Fprintf(w, "%s\t%s\t%s\t%+.2f\t%s\n",
			m.Metric, formatMetric(m.A), formatMetric(m.B), m.Delta, formatPctChange(m.PctChange))
	}
	w.Flush()
}

func printCompare(items []aibom.AIBOM) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)

	names := make([]string, len(items))
	for i, a := range items {
		names[i] = a.Name
	}
	fmt.Fprintln(w, "FIELD\t"+strings.Join(names, "\t"))

	row := func(label string, values func(aibom.AIBOM) string) {
		cells := make([]string, len(items))
		for i, a := range items {
			cells[i] = values(a)
		}
		fmt.Fprintln(w, label+"\t"+strings.Join(cells, "\t"))
	}

	row("Model", func(a aibom.AIBOM) string { return a.Data.Model.Name })
	row("Quantization", func(a aibom.AIBOM) string { return a.Data.Model.Quantization })
	row("Intent", func(a aibom.AIBOM) string { return a.ExperimentIntent })
	row("GPU Type", func(a aibom.AIBOM) string { return a.Data.Environment.GPUType })
	fmt.Fprintln(w)

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
	w.Flush()
}
