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
	"time"

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
			switch sortBy {
			case "", "age":
				// Default: List() itself only sorts alphabetically by
				// namespace/name (kept that way since completeAIBOMNames
				// also relies on it, for tab-completion), so `list` applies
				// its own default of oldest-first by age on top of that
				// unless a different --sort-by metric is requested.
				aibom.SortByAge(items, ascending)
			default:
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
	listCmd.Flags().StringVar(&sortBy, "sort-by", "", "rank by a performance metric (gpu-utilization, gpu-memory, gpu-power, cpu-usage, memory-usage, network-rx, network-tx); defaults to 'age' (oldest AIBOM first)")
	listCmd.Flags().BoolVar(&ascending, "ascending", false, "reverse --sort-by order (lowest first; for the default age sort, shows most recently collected first)")

	var brief bool
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
			ctx := context.Background()
			a, err := aibom.Get(ctx, client, namespace, args[0])
			if err != nil {
				return err
			}
			verifyResult := aibom.Verify(ctx, client, a)
			printDescribe(a, verifyResult, brief)
			return nil
		},
	}
	getCmd.Flags().BoolVarP(&brief, "brief", "b", false, "omit pod list, performance detail table, and AIBOM metadata")

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
	"storage_read_throughput", "storage_write_throughput",
}

var telemetryMetricLabels = map[string]string{
	"gpu_utilization":          "GPU Utilization",
	"gpu_memory_used":          "GPU Memory",
	"gpu_power":                "GPU Power",
	"cpu_usage":                "CPU Usage",
	"storage_read_throughput":  "Storage Read",
	"storage_write_throughput": "Storage Write",
	"memory_usage":             "Memory Usage",
	"network_receive":          "Network RX",
	"network_transmit":         "Network TX",
}

// colorizeShape wraps each arrow rune in a Sparkline() shape with its own
// direction's color (green for ↗, red for ↘, yellow for →), rather than one
// color for the whole string based on the metric's overall Trend(). A mixed
// shape like "↘↗" (dip then recover) needs both colors -- painting the whole
// thing one color (e.g. yellow for Trend()'s "volatile" verdict) would make
// the same ↘ glyph appear red in one row (a metric classified "down") and
// yellow in another (a metric classified "volatile"), which reads as
// inconsistent even though both rows show a real decline at that point.
func colorizeShape(shape string) string {
	var b strings.Builder
	for _, r := range shape {
		switch r {
		case '↗':
			b.WriteString(green(string(r)))
		case '↘':
			b.WriteString(red(string(r)))
		case '→':
			b.WriteString(yellow(string(r)))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatSegment(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *v)
}

func printList(items []aibom.AIBOM, allNamespaces bool, sortBy string) {
	metricHeader, metricGet := "", aibom.SortableMetrics[sortBy]
	if sortBy != "" && sortBy != "age" {
		metricHeader = metricLabels[sortBy]
	}

	cols := []string{"NAME", "JOB", "MODEL", "INTENT", "QUANTIZATION", "GPU TYPE", "AGE", "STATUS"}
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
			a.Data.Model.Quantization, a.Data.Environment.GPUType, humanAge(a.CollectedAt),
		}
		if allNamespaces {
			values = append([]string{a.Namespace}, values...)
		}
		row := plainRow(values...)
		row = append(row, statusCell(a.Data.ExecutionMetadata.Status, nil))
		if metricGet != nil {
			row = append(row, plainCell(formatMetric(metricGet(a.Data.ResourceUtilization))))
		}
		rows = append(rows, row)
	}
	writeTable(os.Stdout, rows)
	if len(items) == 0 {
		fmt.Println("No AIBOMs found.")
	}
}

// humanAge renders the time elapsed since collectedAt (an RFC3339
// timestamp) the same way kubectl's own AGE column does for creation
// timestamps -- a single coarse unit (42s, 12m, 3h, 5d) rather than the raw
// timestamp, so a `list` row can be scanned at a glance instead of parsed.
// Falls back to the ISO date once a record is old enough that a relative
// unit stops being useful, and to "-" if collectedAt is missing/unparseable.
func humanAge(collectedAt string) string {
	t, err := time.Parse(time.RFC3339, collectedAt)
	if err != nil {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// statusCell renders a pod/execution-metadata status colored by severity:
// red for OOMKilled (the specific failure this was added to surface -- see
// CLAUDE.md's Pod Termination Status section), yellow for any other
// non-Completed status, green for Completed, and "-" when the watcher never
// captured a container status at all (not the same as a clean exit).
// exitCode is only appended when present -- the job-level rollup
// (ExecutionMetadata.Status) never carries one, since it's a single value
// merged across every pod, not a signal from one specific container.
func statusCell(status string, exitCode *int) cell {
	if status == "" {
		return plainCell("-")
	}
	text := status
	if exitCode != nil {
		text = fmt.Sprintf("%s (exit %d)", status, *exitCode)
	}
	switch status {
	case "OOMKilled":
		return coloredCell(text, red(text))
	case "Completed":
		return coloredCell(text, green(text))
	default:
		return coloredCell(text, yellow(text))
	}
}

// formatPodStatus is statusCell's rendered text for plain (non-table)
// output, e.g. printDescribe's fmt.Printf lines, which don't need visible
// vs. rendered width tracking the way writeTable's columns do.
func formatPodStatus(status string, exitCode *int) string {
	return statusCell(status, exitCode).rendered
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

// formatVerify renders a Verify result the way printDescribe's other status
// lines are rendered: green only for the one status that actually means
// "trust this," yellow for anything that needs a human to look closer, red
// for a confirmed mismatch, and plain (uncolored) for the common
// not-signed-yet case, which isn't itself a bad sign.
func formatVerify(r aibom.VerifyResult) string {
	switch r.Status {
	case aibom.VerifyValid:
		return green("✓ Verified") + " (signature matches, key confirmed against cluster)"
	case aibom.VerifyUnsigned:
		return "not signed"
	case aibom.VerifyUnconfirmed:
		return yellow("signed (unconfirmed)") + " — " + r.Detail
	case aibom.VerifyKeyMismatch:
		return yellow("⚠ KEY MISMATCH") + " — " + r.Detail
	case aibom.VerifyInvalid:
		return red("✗ INVALID SIGNATURE") + " — " + r.Detail
	default:
		return r.Detail
	}
}

func printDescribe(a aibom.AIBOM, verifyResult aibom.VerifyResult, brief bool) {
	fmt.Printf("Name:              %s\n", a.Name)
	fmt.Printf("Namespace:         %s\n", a.Namespace)
	fmt.Printf("Job:               %s\n", a.JobName)
	if a.Data.ExperimentName != "" && a.Data.ExperimentName != a.JobName {
		fmt.Printf("Experiment:        %s\n", a.Data.ExperimentName)
	}
	if a.Data.ExperimentDescription != "" {
		fmt.Printf("Description:       %s\n", a.Data.ExperimentDescription)
	}
	fmt.Printf("Experiment Intent: %s\n", a.ExperimentIntent)
	fmt.Printf("Runtime:           %s (%s -> %s)\n",
		a.Data.ExecutionMetadata.Duration(),
		a.Data.ExecutionMetadata.EarliestPodStart(),
		a.CollectedAt,
	)
	fmt.Printf("Status:            %s\n", formatPodStatus(a.Data.ExecutionMetadata.Status, nil))
	fmt.Printf("Signature:         %s\n", formatVerify(verifyResult))
	fmt.Println()
	fmt.Println(bold("Model:"))
	fmt.Printf("  Name:          %s\n", a.Data.Model.Name)
	fmt.Printf("  Version:       %s\n", a.Data.Model.Version)
	fmt.Printf("  Architecture:  %s\n", a.Data.Model.Architecture)
	fmt.Printf("  Framework:     %s\n", a.Data.Model.Framework)
	fmt.Printf("  Dtype:         %s\n", a.Data.Model.Dtype)
	fmt.Printf("  Quantization:  %s (%d-bit)\n", a.Data.Model.Quantization, a.Data.Model.QuantizationBits)
	if sd := a.Data.Model.SpeculativeDecoding; sd != nil {
		fmt.Printf("  Speculative Decoding: %s (draft: %s, tokens: %d)\n", boolStr(sd.Enabled), sd.DraftModel, sd.NumSpeculativeTokens)
	}
	fmt.Println()
	fmt.Println(bold("Dataset:"))
	fmt.Printf("  Declared:      %s %s (license: %s, via: %s)\n",
		a.Data.Dataset.Declared.Name, a.Data.Dataset.Declared.Version,
		a.Data.Dataset.Declared.License, a.Data.Dataset.Declared.DeclaredVia)
	for _, d := range a.Data.Dataset.AutoDetected {
		match := green("matches declared")
		if !d.MatchesDeclared {
			match = red("DOES NOT MATCH DECLARED")
		}
		fmt.Printf("  Auto-detected: %s %s (license: %s, seen via: %s) — %s\n", d.DatasetName, d.Version, d.License, strings.Join(d.SeenVia, ", "), match)
	}
	fmt.Println()
	fmt.Println(bold("Source:"))
	fmt.Printf("  Repository:    %s\n", a.Data.SourceCode.GitRepository)
	dirty := fmt.Sprintf("%v", a.Data.SourceCode.Dirty)
	if a.Data.SourceCode.Dirty {
		dirty = yellow(dirty)
	}
	fmt.Printf("  Commit:        %s (branch: %s, dirty: %s, via: %s)\n",
		a.Data.SourceCode.GitCommit, a.Data.SourceCode.GitBranch, dirty, a.Data.SourceCode.DeclaredVia)

	if t := a.Data.Training; t != nil {
		fmt.Println()
		fmt.Println(bold("Training:"))
		fmt.Printf("  Optimizer:              %s\n", t.Optimizer)
		fmt.Printf("  Learning Rate:          %v\n", t.LearningRate)
		fmt.Printf("  Batch Size:             %v\n", t.BatchSize)
		fmt.Printf("  Epochs:                 %v\n", t.Epochs)
		fmt.Printf("  Random Seed:            %v\n", t.RandomSeed)
		fmt.Printf("  Parallelization:        %s\n", t.ParallelizationStrategy)
	}
	if ft := a.Data.FineTuning; ft != nil {
		fmt.Println()
		fmt.Println(bold("Fine-Tuning:"))
		fmt.Printf("  Adaptation Method: %s\n", ft.AdaptationMethod)
		fmt.Printf("  LoRA Rank/Alpha:   %v / %v\n", ft.LoRARank, ft.LoRAAlpha)
	}
	if inf := a.Data.Inference; inf != nil {
		fmt.Println()
		fmt.Println(bold("Inference:"))
		fmt.Printf("  Serving Engine:       %s\n", inf.ServingEngine)
		fmt.Printf("  Max Model Len:        %v\n", inf.MaxModelLen)
		fmt.Printf("  Tensor/Pipeline/Data Parallel: %v / %v / %v\n", inf.TensorParallelSize, inf.PipelineParallelSize, inf.DataParallelSize)
		fmt.Printf("  Expert Parallel:      %s\n", boolStr(inf.EnableExpertParallel))
		fmt.Printf("  GPU Memory Util:      %v\n", inf.GPUMemoryUtilization)
		fmt.Printf("  Temperature/TopP/TopK: %v / %v / %v\n", inf.Temperature, inf.TopP, inf.TopK)
		fmt.Printf("  Max Tokens:           %d\n", inf.MaxTokens)
	}

	fmt.Println()
	fmt.Println("Environment:")
	fmt.Printf("  GPU:           %s x%d\n", a.Data.Environment.GPUType, a.Data.Environment.GPUCount)
	fmt.Printf("  CPU:           %s x%d\n", a.Data.Environment.CPUModel, a.Data.Environment.CPUCores)
	fmt.Printf("  Memory:        %.2f GB (%d NUMA node(s))\n", a.Data.Environment.MemoryGB, a.Data.Environment.NUMANodes)
	fmt.Printf("  CUDA/Driver:   %s / %s\n", a.Data.Environment.CUDAVersion, a.Data.Environment.DriverVersion)
	fmt.Printf("  Framework:     %s\n", a.Data.Environment.FrameworkVersion)
	fmt.Printf("  Kernel:        %s\n", a.Data.Environment.KernelVersion)

	if !brief {
		fmt.Println()
		fmt.Println(bold("Pods:"))
		for _, p := range a.Data.ExecutionMetadata.Pods {
			fmt.Printf("  %s  node=%s  ip=%s  start=%s  status=%s\n",
				p.PodName, p.NodeName, p.PodIP, p.StartTime, formatPodStatus(p.Status, p.ExitCode))
		}
	}

	fmt.Println()
	fmt.Println("Performance:")
	ru := a.Data.ResourceUtilization
	if ru.Note != "" {
		fmt.Printf("  %s\n", ru.Note)
	} else {
		for _, key := range telemetryMetricOrder {
			m, ok := ru.Metrics[key]
			if !ok {
				continue
			}
			fmt.Printf("  %-16s %.2f %s\n", telemetryMetricLabels[key]+":", m.Avg, m.Unit)
		}
		if ru.SummaryIncludesColdStart {
			fmt.Println("  (includes cold start)")
		}
		for _, link := range ru.GrafanaLinks {
			fmt.Printf("  Grafana:         %s\n", link)
		}
		if !brief {
			printMetricDetail(ru)
		}
	}

	if !brief {
		fmt.Println()
		fmt.Println(bold("AIBOM Metadata:"))
		fmt.Printf("  Version:            %s\n", a.Data.Metadata.AIBOMVersion)
		fmt.Printf("  Generated At:       %s\n", a.Data.Metadata.GeneratedAt)
		fmt.Printf("  Generator:          %s\n", a.Data.Metadata.Generator)
		fmt.Printf("  Schema Compliance:  %s\n", a.Data.Metadata.SchemaCompliance)
		fmt.Printf("  Dataset Detection:  %s\n", a.Data.Metadata.DatasetDetection)
	}
}

func boolStr(b bool) string {
	return fmt.Sprintf("%v", b)
}

// printMetricDetail prints the min/max/p95 and within-run shape for each
// metric in ru.Metrics -- detail a flat average can't show, e.g. whether GPU
// utilization held steady or throttled down partway through the run. Silent
// no-op if the AIBOM predates this field (an older postprocess.py). Rendered
// as a table (not manually padded Printf columns) since the values span
// wildly different magnitudes across metrics (e.g. "28.00" vs "20500.00"),
// which fixed-width padding can't keep aligned.
func printMetricDetail(ru aibom.ResourceUtilization) {
	if len(ru.Metrics) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(bold("Performance Detail:"))
	rows := [][]cell{headerRow("METRIC", "MIN", "AVG", "MAX", "LIMIT", "P95", "UNIT", "1ST -> MID -> LAST", "SHAPE")}
	for _, key := range telemetryMetricOrder {
		m, ok := ru.Metrics[key]
		if !ok {
			continue
		}
		segmentsText := fmt.Sprintf(
			"%s -> %s -> %s",
			formatSegment(m.Segments.FirstThird), formatSegment(m.Segments.MiddleThird), formatSegment(m.Segments.LastThird),
		)
		rows = append(rows, []cell{
			labelCell(telemetryMetricLabels[key]),
			plainCell(formatMetric(m.Min)),
			plainCell(formatMetric(m.Avg)),
			maxCell(m.Max, m.Limit),
			limitCell(m.Limit),
			plainCell(formatMetric(m.P95)),
			plainCell(m.Unit),
			plainCell(segmentsText),
			shapeCell(m.Segments.Sparkline()),
		})
	}
	writeTable(os.Stdout, rows)
}

// limitCell renders a metric's configured resource limit, or "-" for a
// metric with no limit concept (GPU/network/storage) or one where no
// container in the workload actually set one.
func limitCell(limit *float64) cell {
	if limit == nil {
		return plainCell("-")
	}
	return plainCell(formatMetric(*limit))
}

// maxCell colors MAX by how close it ran to the metric's limit (when one is
// present) -- red at/above the limit (a near-certain contributor to
// whatever killed the pod; cross-reference the pod's own Status), yellow
// within 10% of it (a near-miss worth noting even on a Completed pod, since
// no explicit status field would otherwise surface it -- see CLAUDE.md's
// Segmented Performance Stats section), plain otherwise.
func maxCell(max float64, limit *float64) cell {
	text := formatMetric(max)
	if limit == nil || *limit <= 0 {
		return plainCell(text)
	}
	switch pct := max / *limit; {
	case pct >= 1.0:
		return coloredCell(text, red(text))
	case pct >= 0.9:
		return coloredCell(text, yellow(text))
	default:
		return plainCell(text)
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
			metricCellWithShape(m.A, m.ShapeA),
			metricCellWithShape(m.B, m.ShapeB),
			coloredCell(deltaText, colorBySign(m.Delta, deltaText)),
			coloredCell(changeText, colorBySign(m.PctChange, changeText)),
		})
	}
	writeTable(os.Stdout, rows)
}

// metricCellWithShape appends a within-run arrow sparkline (see
// MetricSegments.Sparkline) to a metric value, when known -- e.g. "75.00 ↘↗"
// flags that a run's own average dipped then recovered, a distinction the
// cross-run delta/change columns can't make on their own.
func metricCellWithShape(v float64, shape string) cell {
	if shape == "" {
		return plainCell(formatMetric(v))
	}
	return joinShapeCell(formatMetric(v), shape)
}

// shapeCell renders a bare arrow sparkline (see MetricSegments.Sparkline)
// as its own cell, e.g. for printMetricDetail's SHAPE column.
func shapeCell(shape string) cell {
	if shape == "" {
		return plainCell("")
	}
	return joinShapeCell("", shape)
}

// joinShapeCell joins text and shape with a space (or returns shape alone if
// text is empty), colorizing each arrow in shape individually (see
// colorizeShape). Built as a cell (not a plain string) so the color escape
// codes don't get counted by writeTable's column-width calculation.
func joinShapeCell(text, shape string) cell {
	visible, rendered := shape, colorizeShape(shape)
	if text != "" {
		visible = text + " " + visible
		rendered = text + " " + rendered
	}
	return coloredCell(visible, rendered)
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
			cells[i+1] = runCell(i, values(a))
		}
		rows = append(rows, cells)
	}

	row("Model", func(a aibom.AIBOM) string { return a.Data.Model.Name })
	row("Quantization", func(a aibom.AIBOM) string { return a.Data.Model.Quantization })
	row("Intent", func(a aibom.AIBOM) string { return a.ExperimentIntent })
	row("GPU Type", func(a aibom.AIBOM) string { return a.Data.Environment.GPUType })
	writeTable(os.Stdout, rows)
	fmt.Println()

	fmt.Println(bold("Performance:"))
	metricHeader := make([]cell, len(items)+1)
	metricHeader[0] = labelHeaderCell("METRIC")
	for i, a := range items {
		metricHeader[i+1] = runHeaderCell(i, a.Name)
	}
	rows = [][]cell{metricHeader}
	for _, m := range []struct {
		label string
		get   func(aibom.ResourceUtilization) float64
	}{
		{"GPU Utilization %", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("gpu_utilization") }},
		{"GPU Memory (MiB)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("gpu_memory_used") }},
		{"GPU Power (W)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("gpu_power") }},
		{"CPU Usage (cores)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("cpu_usage") }},
		{"Memory Usage (GB)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("memory_usage") }},
		{"Network RX (Mbps)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("network_receive") }},
		{"Network TX (Mbps)", func(r aibom.ResourceUtilization) float64 { return r.MetricAvg("network_transmit") }},
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
