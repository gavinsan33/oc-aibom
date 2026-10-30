// Command kubectl-aibom is a kubectl/oc plugin (invoked as `kubectl aibom`
// or `oc aibom`) that makes it easier to list, inspect, and compare AIBOM
// custom resources (group aibom.io) than raw `oc get aibom -o yaml`.
package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

	// The no-arg root (interactive) and `list` share the same filter/sort
	// flags, each with its own copy so cobra can bind them independently.
	var rootOpts, listOpts listFilterFlags
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "get"},
		Short:   "List AIBOMs, optionally filtered by model/intent/quantization",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			items, err := listOpts.load(context.Background(), client, namespace)
			if err != nil {
				return err
			}
			printList(items, listOpts.allNamespaces, listOpts.sortBy)
			return nil
		},
	}
	listOpts.register(listCmd)

	// With no subcommand, `oc aibom` opens the interactive browser on a
	// terminal, falling back to the plain `list` output when stdout is piped
	// or redirected (so `oc aibom | grep` still works). Explicit subcommands
	// are always print-only.
	root.Args = cobra.NoArgs
	root.RunE = func(cmd *cobra.Command, args []string) error {
		client, namespace, err := buildClient(configFlags)
		if err != nil {
			return err
		}
		items, err := rootOpts.load(context.Background(), client, namespace)
		if err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			printList(items, rootOpts.allNamespaces, rootOpts.sortBy)
			return nil
		}
		return runListTUI(items, client, rootOpts.allNamespaces, rootOpts.sortBy, rootOpts.ascending)
	}
	rootOpts.register(root)

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
			printDescribe(os.Stdout, a, verifyResult, aibom.VerifySeries(ctx, client, a, verifyResult), brief)
			return nil
		},
	}
	getCmd.Flags().BoolVarP(&brief, "brief", "b", false, "omit pod list, performance detail table, and AIBOM metadata")

	diffCmd := &cobra.Command{
		Use:               "diff <name-a> <name-b>",
		Short:             "Show only the fields that differ between two AIBOMs (compare --changed)",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAIBOMNames(configFlags, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			items, err := getAIBOMs(context.Background(), client, namespace, args)
			if err != nil {
				return err
			}
			printCompare(os.Stdout, items, true)
			return nil
		},
	}

	var compareChanged bool
	compareCmd := &cobra.Command{
		Use:               "compare <name> <name> [<name>...]",
		Short:             "Compare two or more AIBOMs field by field, with performance side by side",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeAIBOMNames(configFlags, 0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			items, err := getAIBOMs(context.Background(), client, namespace, args)
			if err != nil {
				return err
			}
			printCompare(os.Stdout, items, compareChanged)
			return nil
		},
	}
	compareCmd.Flags().BoolVar(&compareChanged, "changed", false, "show only the fields that differ across the compared AIBOMs")

	var graphMetrics []string
	var graphPods bool
	var graphWidth int
	var graphText bool
	graphCmd := &cobra.Command{
		Use:               "graph <name> [<name>...]",
		Short:             "Chart AIBOMs' stored telemetry time series in the terminal, overlaid for comparison",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeAIBOMNames(configFlags, 0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, namespace, err := buildClient(configFlags)
			if err != nil {
				return err
			}
			ctx := context.Background()
			items, err := getAIBOMs(ctx, client, namespace, args)
			if err != nil {
				return err
			}
			runs, notes := buildGraphRuns(ctx, client, items)
			printNotes := func() { fmt.Fprintln(os.Stderr, strings.Join(notes, "\n")) }
			if len(runs) == 0 {
				printNotes()
				return fmt.Errorf("no usable stored telemetry for the given AIBOMs")
			}
			// Interactive full-screen view on a terminal; plain sparklines when
			// piped or with --text. Notes go after the TUI so they survive the
			// alternate screen.
			if !graphText && term.IsTerminal(int(os.Stdout.Fd())) {
				keys, err := graphMetricKeys(runs, graphMetrics)
				if err != nil {
					return err
				}
				err = runGraphTUI(runs, keys, graphPods)
				printNotes()
				return err
			}
			printNotes()
			width := graphWidth
			if width <= 0 {
				width = 80
				if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 20 {
					width = w - 2
				}
			}
			return printGraph(os.Stdout, runs, graphMetrics, graphPods, width)
		},
	}
	graphCmd.Flags().StringSliceVar(&graphMetrics, "metric", nil, "only graph these metric keys (e.g. gpu_utilization,memory_usage); default all")
	graphCmd.Flags().BoolVar(&graphPods, "pods", false, "draw each run's pod/GPU/container lines instead of its aggregate")
	graphCmd.Flags().BoolVar(&graphText, "text", false, "print sparklines instead of opening the interactive full-screen view (automatic when stdout isn't a terminal)")
	graphCmd.Flags().IntVar(&graphWidth, "width", 0, "text-mode graph width in columns (default: terminal width)")

	root.AddCommand(listCmd, getCmd, diffCmd, compareCmd, graphCmd)

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

// getAIBOMs fetches the named AIBOMs in order, failing on the first missing
// one (shared by compare, diff, and graph).
func getAIBOMs(ctx context.Context, client dynamic.Interface, namespace string, names []string) ([]aibom.AIBOM, error) {
	items := make([]aibom.AIBOM, 0, len(names))
	for _, name := range names {
		a, err := aibom.Get(ctx, client, namespace, name)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, nil
}

// listFilterFlags is the filter/sort state shared by the `list` subcommand
// and the no-arg interactive root, registered on each via register.
type listFilterFlags struct {
	allNamespaces bool
	driftOnly     bool
	ascending     bool
	sortBy        string
	f             aibom.Filter
}

func (o *listFilterFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.BoolVarP(&o.allNamespaces, "all-namespaces", "A", false, "list AIBOMs across all namespaces")
	fl.StringVar(&o.f.Model, "model", "", "filter by model.name")
	fl.StringVar(&o.f.Intent, "intent", "", "filter by experiment intent (training|sft|inference)")
	fl.StringVar(&o.f.Quantization, "quantization", "", "filter by model.quantization")
	fl.StringVar(&o.f.Architecture, "architecture", "", "filter by model.architecture")
	fl.StringVar(&o.f.Framework, "framework", "", "filter by model.framework")
	fl.StringVar(&o.f.GPUType, "gpu-type", "", "filter by environment.gpu_type")
	fl.StringVar(&o.f.JobName, "job", "", "filter by job name")
	fl.StringVar(&o.f.GitBranch, "git-branch", "", "filter by source_code.git_branch")
	fl.StringVar(&o.f.GitRepository, "git-repository", "", "filter by source_code.git_repository")
	fl.StringVar(&o.f.ServingEngine, "serving-engine", "", "filter by inference.serving_engine")
	fl.StringVar(&o.f.AdaptationMethod, "adaptation-method", "", "filter by fine_tuning.adaptation_method")
	fl.StringVar(&o.f.Optimizer, "optimizer", "", "filter by training.optimizer")
	fl.BoolVar(&o.driftOnly, "drift-only", false, "only show AIBOMs where auto-detected dataset(s) disagree with the declared dataset")
	fl.StringVar(&o.sortBy, "sort-by", "", "rank by a performance metric (gpu-utilization, gpu-memory, gpu-power, cpu-usage, memory-usage, network-rx, network-tx); defaults to 'age' (oldest AIBOM first)")
	fl.BoolVar(&o.ascending, "ascending", false, "reverse --sort-by order (lowest first; for the default age sort, shows most recently collected first)")
}

// load fetches, filters, and sorts AIBOMs per the flags. The default sort is
// oldest-first by age (List itself only sorts by namespace/name, which
// completeAIBOMNames relies on for tab-completion).
func (o *listFilterFlags) load(ctx context.Context, client dynamic.Interface, namespace string) ([]aibom.AIBOM, error) {
	ns := namespace
	if o.allNamespaces {
		ns = ""
	}
	items, err := aibom.List(ctx, client, ns)
	if err != nil {
		return nil, err
	}
	items = aibom.Apply(items, o.f)
	if o.driftOnly {
		items = aibom.DriftOnly(items)
	}
	switch o.sortBy {
	case "", "age":
		aibom.SortByAge(items, o.ascending)
	default:
		if err := aibom.SortByMetric(items, o.sortBy, o.ascending); err != nil {
			return nil, err
		}
	}
	return items, nil
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

// servingMetricOrder and servingMetricLabels are inference.performance's analog of
// telemetryMetricOrder/telemetryMetricLabels above -- the serving engine's own
// metrics (aibom-webhook-service's SERVING_TELEMETRY_QUERIES), not
// DCGM/cAdvisor hardware metrics.
var servingMetricOrder = []string{
	"time_to_first_token_seconds", "inter_token_latency_seconds",
	"num_requests_running", "num_requests_waiting",
	"kv_cache_usage", "prompt_throughput", "generation_throughput",
}

var servingMetricLabels = map[string]string{
	"time_to_first_token_seconds": "TTFT",
	"inter_token_latency_seconds": "ITL",
	"num_requests_running":        "Requests Running",
	"num_requests_waiting":        "Requests Waiting",
	"kv_cache_usage":              "KV Cache Usage",
	"prompt_throughput":           "Prompt Throughput",
	"generation_throughput":       "Gen Throughput",
}

func formatSegment(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *v)
}

func printList(items []aibom.AIBOM, allNamespaces bool, sortBy string) {
	writeTable(os.Stdout, listRows(items, allNamespaces, sortBy))
	if len(items) == 0 {
		fmt.Println("No AIBOMs found.")
	}
}

// listRows builds the `list` table (header row first) as [][]cell, so both
// printList and the interactive browser render identical columns.
func listRows(items []aibom.AIBOM, allNamespaces bool, sortBy string) [][]cell {
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
	return rows
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

// formatSeriesVerify renders a VerifySeries result with the same color rules
// as formatVerify: green only when the series is authenticated end to end,
// red for a confirmed digest mismatch, yellow for anything a human should
// look at. Callers skip SeriesAbsent (an AIBOM without a stored series isn't
// worth a line).
func formatSeriesVerify(r aibom.SeriesResult) string {
	switch r.Status {
	case aibom.SeriesVerified:
		return green("✓ Verified") + " (matches the digest in the AIBOM's verified signed data)"
	case aibom.SeriesDigestOnly:
		return yellow("digest matches (AIBOM not verified)") + " — " + r.Detail
	case aibom.SeriesMismatch:
		return red("✗ DIGEST MISMATCH") + " — " + r.Detail
	case aibom.SeriesMissing:
		return yellow("missing") + " — " + r.Detail
	case aibom.SeriesUnconfirmed:
		return yellow("unconfirmed") + " — " + r.Detail
	default:
		return r.Detail
	}
}

func printDescribe(w io.Writer, a aibom.AIBOM, verifyResult aibom.VerifyResult, seriesResult aibom.SeriesResult, brief bool) {
	fmt.Fprintf(w, "Name:              %s\n", a.Name)
	fmt.Fprintf(w, "Namespace:         %s\n", a.Namespace)
	fmt.Fprintf(w, "Job:               %s\n", a.JobName)
	if a.Data.ExperimentName != "" && a.Data.ExperimentName != a.JobName {
		fmt.Fprintf(w, "Experiment:        %s\n", a.Data.ExperimentName)
	}
	if a.Data.ExperimentDescription != "" {
		fmt.Fprintf(w, "Description:       %s\n", a.Data.ExperimentDescription)
	}
	if via := a.Data.ExperimentIntentDeclaredVia; via != "" {
		fmt.Fprintf(w, "Experiment Intent: %s (via: %s)\n", a.ExperimentIntent, via)
	} else {
		fmt.Fprintf(w, "Experiment Intent: %s\n", a.ExperimentIntent)
	}
	fmt.Fprintf(w, "Runtime:           %s (%s -> %s)\n",
		a.Data.ExecutionMetadata.Duration(),
		a.Data.ExecutionMetadata.EarliestPodStart(),
		a.CollectedAt,
	)
	fmt.Fprintf(w, "Status:            %s\n", formatPodStatus(a.Data.ExecutionMetadata.Status, nil))
	fmt.Fprintf(w, "Signature:         %s\n", formatVerify(verifyResult))
	if seriesResult.Status != aibom.SeriesAbsent {
		fmt.Fprintf(w, "Telemetry Series:  %s\n", formatSeriesVerify(seriesResult))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Model:"))
	if via := a.Data.Model.NameDeclaredVia; via != "" {
		fmt.Fprintf(w, "  Name:          %s (via: %s)\n", a.Data.Model.Name, via)
	} else {
		fmt.Fprintf(w, "  Name:          %s\n", a.Data.Model.Name)
	}
	fmt.Fprintf(w, "  Version:       %s\n", a.Data.Model.Version)
	if a.Data.Model.Revision != "" {
		fmt.Fprintf(w, "  Revision:      %s\n", a.Data.Model.Revision)
	}
	if a.Data.Model.BaseModel != "" {
		fmt.Fprintf(w, "  Base Model:    %s\n", a.Data.Model.BaseModel)
	}
	if a.Data.Model.SizeBytes > 0 {
		fmt.Fprintf(w, "  Size:          %.1f GB\n", float64(a.Data.Model.SizeBytes)/1e9)
	}
	fmt.Fprintf(w, "  Architecture:  %s\n", a.Data.Model.Architecture)
	fmt.Fprintf(w, "  Framework:     %s\n", a.Data.Model.Framework)
	fmt.Fprintf(w, "  Dtype:         %s\n", a.Data.Model.Dtype)
	fmt.Fprintf(w, "  Quantization:  %s (%d-bit)\n", a.Data.Model.Quantization, a.Data.Model.QuantizationBits)
	if sd := a.Data.Model.SpeculativeDecoding; sd != nil {
		fmt.Fprintf(w, "  Speculative Decoding: %s (draft: %s, tokens: %d)\n", boolStr(sd.Enabled), sd.DraftModel, sd.NumSpeculativeTokens)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Dataset:"))
	if a.Data.Dataset.Declared.Name == "" && len(a.Data.Dataset.AutoDetected) == 0 {
		// Inference AIBOMs usually have no dataset block at all.
		fmt.Fprintln(w, "  "+faint("No dataset was detected for this workload."))
	} else {
		fmt.Fprintf(w, "  Declared:      %s %s (license: %s, via: %s)\n",
			a.Data.Dataset.Declared.Name, a.Data.Dataset.Declared.Version,
			a.Data.Dataset.Declared.License, a.Data.Dataset.Declared.DeclaredVia)
	}
	for _, d := range a.Data.Dataset.AutoDetected {
		var match string
		switch {
		case d.MatchesDeclared == nil:
			match = faint("not reconciled (declared inferred from runtime)")
		case *d.MatchesDeclared:
			match = green("matches declared")
		default:
			match = red("DOES NOT MATCH DECLARED")
		}
		fmt.Fprintf(w, "  Auto-detected: %s %s (license: %s, seen via: %s) — %s\n", d.DatasetName, d.Version, d.License, strings.Join(d.SeenVia, ", "), match)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Source:"))
	fmt.Fprintf(w, "  Repository:    %s\n", a.Data.SourceCode.GitRepository)
	dirty := fmt.Sprintf("%v", a.Data.SourceCode.Dirty)
	if a.Data.SourceCode.Dirty {
		dirty = yellow(dirty)
	}
	fmt.Fprintf(w, "  Commit:        %s (branch: %s, dirty: %s, via: %s)\n",
		a.Data.SourceCode.GitCommit, a.Data.SourceCode.GitBranch, dirty, a.Data.SourceCode.DeclaredVia)

	if t := a.Data.Training; t != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("Training:"))
		fmt.Fprintf(w, "  Optimizer:              %s\n", t.Optimizer)
		fmt.Fprintf(w, "  Learning Rate:          %v\n", t.LearningRate)
		fmt.Fprintf(w, "  Batch Size:             %v\n", t.BatchSize)
		fmt.Fprintf(w, "  Epochs:                 %v\n", t.Epochs)
		fmt.Fprintf(w, "  Random Seed:            %v\n", t.RandomSeed)
		fmt.Fprintf(w, "  Parallelization:        %s\n", t.ParallelizationStrategy)
	}
	if ft := a.Data.FineTuning; ft != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("Fine-Tuning:"))
		fmt.Fprintf(w, "  Adaptation Method: %s\n", ft.AdaptationMethod)
		fmt.Fprintf(w, "  LoRA Rank/Alpha:   %v / %v\n", ft.LoRARank, ft.LoRAAlpha)
	}
	if inf := a.Data.Inference; inf != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("Inference:"))
		fmt.Fprintf(w, "  Serving Engine:       %s\n", inf.ServingEngine)
		fmt.Fprintf(w, "  Max Model Len:        %v\n", inf.MaxModelLen)
		fmt.Fprintf(w, "  Tensor/Pipeline/Data Parallel: %v / %v / %v\n", inf.TensorParallelSize, inf.PipelineParallelSize, inf.DataParallelSize)
		fmt.Fprintf(w, "  Expert Parallel:      %s\n", boolStr(inf.EnableExpertParallel))
		fmt.Fprintf(w, "  GPU Memory Util:      %v\n", inf.GPUMemoryUtilization)
		fmt.Fprintf(w, "  Temperature/TopP/TopK: %v / %v / %v\n", inf.Temperature, inf.TopP, inf.TopK)
		fmt.Fprintf(w, "  Max Tokens:           %d\n", inf.MaxTokens)
		if inf.ServedModelName != "" {
			fmt.Fprintf(w, "  Served Model Name:    %s\n", inf.ServedModelName)
		}
		for _, o := range []struct {
			label string
			v     any
		}{
			{"Max Num Seqs", inf.MaxNumSeqs}, {"Seed", inf.Seed},
			{"Trust Remote Code", inf.TrustRemoteCode}, {"Enforce Eager", inf.EnforceEager},
			{"Prefix Caching", inf.EnablePrefixCaching},
		} {
			if o.v != nil {
				fmt.Fprintf(w, "  %-21s %v\n", o.label+":", o.v)
			}
		}
		if perf := inf.Performance; perf != nil && len(perf.Metrics) > 0 {
			printMetricsSection(w, "Inference Performance", perf.Metrics, servingMetricOrder, servingMetricLabels, perf.SummaryIncludesColdStart, brief)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintf(w, "  GPU:           %s x%d\n", a.Data.Environment.GPUType, a.Data.Environment.GPUCount)
	fmt.Fprintf(w, "  CPU:           %s x%d\n", a.Data.Environment.CPUModel, a.Data.Environment.CPUCores)
	fmt.Fprintf(w, "  Memory:        %.2f GB (%d NUMA node(s))\n", a.Data.Environment.MemoryGB, a.Data.Environment.NUMANodes)
	fmt.Fprintf(w, "  CUDA/Driver:   %s / %s\n", a.Data.Environment.CUDAVersion, a.Data.Environment.DriverVersion)
	fmt.Fprintf(w, "  Framework:     %s\n", a.Data.Environment.FrameworkVersion)
	fmt.Fprintf(w, "  Kernel:        %s\n", a.Data.Environment.KernelVersion)
	for _, l := range environmentExtras(a.Data.Environment) {
		fmt.Fprintf(w, "  %-14s %s\n", l[0]+":", l[1])
	}

	if !brief {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("Pods:"))
		for _, p := range a.Data.ExecutionMetadata.Pods {
			fmt.Fprintf(w, "  %s  node=%s  ip=%s  start=%s  status=%s\n",
				p.PodName, p.NodeName, p.PodIP, p.StartTime, formatPodStatus(p.Status, p.ExitCode))
		}
	}

	ru := a.Data.ResourceUtilization
	if ru.Note != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("Hardware Performance:"))
		fmt.Fprintf(w, "  %s\n", ru.Note)
	} else {
		printMetricsSection(w, "Hardware Performance", ru.Metrics, telemetryMetricOrder, telemetryMetricLabels, ru.SummaryIncludesColdStart, brief)
	}

	if !brief {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold("AIBOM Metadata:"))
		fmt.Fprintf(w, "  Version:            %s\n", a.Data.Metadata.AIBOMVersion)
		fmt.Fprintf(w, "  Generated At:       %s\n", a.Data.Metadata.GeneratedAt)
		fmt.Fprintf(w, "  Generator:          %s\n", a.Data.Metadata.Generator)
		fmt.Fprintf(w, "  Schema Compliance:  %s\n", a.Data.Metadata.SchemaCompliance)
		fmt.Fprintf(w, "  Dataset Detection:  %s\n", a.Data.Metadata.DatasetDetection)
	}
}

func boolStr(b bool) string {
	return fmt.Sprintf("%v", b)
}

// printMetricDetail prints the min/max/p95 and within-run shape for each
// metric in metrics -- detail a flat average can't show, e.g. whether GPU
// utilization held steady or throttled down partway through the run. Silent
// no-op if metrics is empty (an AIBOM predating the field, or an inference
// AIBOM with no serving telemetry). Rendered as a table (not manually padded
// Printf columns) since the values span wildly different magnitudes across
// metrics (e.g. "28.00" vs "20500.00"), which fixed-width padding can't keep
// aligned. Generic over order/labels so both ResourceUtilization (hardware
// metrics) and InferencePerformance (the serving engine's metrics) share this
// rendering despite having different known metric-name sets -- see
// telemetryMetricOrder/telemetryMetricLabels vs. servingMetricOrder/servingMetricLabels.
func printMetricDetail(w io.Writer, metrics map[string]aibom.MetricStats, order []string, labels map[string]string) {
	if len(metrics) == 0 {
		return
	}
	rows := [][]cell{headerRow("METRIC", "MIN", "AVG", "MAX", "LIMIT", "P95", "UNIT", "1ST -> MID -> LAST")}
	for _, key := range order {
		m, ok := metrics[key]
		if !ok {
			continue
		}
		segmentsText := fmt.Sprintf(
			"%s -> %s -> %s",
			formatSegment(m.Segments.FirstThird), formatSegment(m.Segments.MiddleThird), formatSegment(m.Segments.LastThird),
		)
		rows = append(rows, []cell{
			labelCell(labels[key]),
			plainCell(formatMetric(m.Min)),
			plainCell(formatMetric(m.Avg)),
			maxCell(m.Max, m.Limit),
			limitCell(m.Limit),
			plainCell(formatMetric(m.P95)),
			plainCell(m.Unit),
			plainCell(segmentsText),
		})
	}
	writeTable(w, rows)
}

// printMetricsSection prints one metrics section -- shared by
// ResourceUtilization ("Hardware Performance") and InferencePerformance's
// ("Inference Performance") identically-shaped map[string]MetricStats.
//
// In brief mode it prints only the per-metric average (the full detail
// table is skipped, so the average is the only view of each metric). In
// full mode the average-only line is redundant with the detail table's AVG
// column, so it's omitted there and the detail table is printed instead.
func printMetricsSection(w io.Writer, title string, metrics map[string]aibom.MetricStats, order []string, labels map[string]string, includesColdStart bool, brief bool) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold(title+":"))
	if brief {
		for _, key := range order {
			m, ok := metrics[key]
			if !ok {
				continue
			}
			fmt.Fprintf(w, "  %-16s %.2f %s\n", labels[key]+":", m.Avg, m.Unit)
		}
	}
	if includesColdStart {
		fmt.Fprintln(w, "  (includes cold start)")
	}
	if !brief {
		printMetricDetail(w, metrics, order, labels)
	}
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

// printCompare renders the console plugin's compare Overview: a Fields table
// (every compared field, one column per run, differing fields flagged) and a
// Hardware Performance table (one column per run, plus Delta/Change columns
// when exactly two runs are compared). With changedOnly, unchanged field rows
// are hidden -- this is what `diff` (a two-run compare --changed) uses.
func printCompare(w io.Writer, items []aibom.AIBOM, changedOnly bool) {
	runHeader := func(label string, extra ...cell) [][]cell {
		h := make([]cell, 0, len(items)+1+len(extra))
		h = append(h, labelHeaderCell(label))
		for i, a := range items {
			h = append(h, runHeaderCell(i, a.Name))
		}
		return [][]cell{append(h, extra...)}
	}

	fmt.Fprintln(w, bold("Fields:"))
	rows := runHeader("FIELD")
	anyDiff := false
	for _, fr := range aibom.CompareFields(items) {
		if changedOnly && !fr.Differs {
			continue
		}
		label := labelCell(fr.Label)
		if fr.Differs {
			anyDiff = true
			marked := fr.Label + " *" // visible even with color disabled
			label = coloredCell(marked, colorize(ansiLightCyan, marked))
		}
		cells := []cell{label}
		for i, v := range fr.Values {
			if v == "" {
				v = "-"
			}
			cells = append(cells, runCell(i, v))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 1 {
		fmt.Fprintln(w, "  (no differing fields)")
	} else {
		writeTable(w, rows)
	}
	if anyDiff {
		fmt.Fprintln(w, faint("  * field differs across runs"))
	}

	twoWay := len(items) == 2
	var extra []cell
	if twoWay {
		extra = []cell{coloredCell("DELTA", bold("DELTA")), coloredCell("CHANGE", bold("CHANGE"))}
	}

	perfTable := func(title string, labels map[string]string, metricRows []aibom.MetricRow) {
		fmt.Fprintln(w)
		fmt.Fprintln(w, bold(title+":"))
		rows := runHeader("METRIC", extra...)
		for _, mr := range metricRows {
			cells := []cell{labelCell(labels[mr.Key])}
			for i := range items {
				text := "-"
				if mr.Available[i] {
					text = strings.TrimSpace(fmt.Sprintf("%s %s", formatMetric(mr.Values[i]), mr.Unit))
				}
				cells = append(cells, runCell(i, text))
			}
			if twoWay {
				deltaText, changeText := "-", "N/A"
				deltaCell, changeCell := plainCell(deltaText), plainCell(changeText)
				if mr.Delta != nil {
					deltaText = fmt.Sprintf("%+.2f", *mr.Delta)
					deltaCell = coloredCell(deltaText, colorBySign(*mr.Delta, deltaText))
				}
				if mr.PctChange != nil {
					changeText = formatPctChange(*mr.PctChange)
					changeCell = coloredCell(changeText, colorBySign(*mr.PctChange, changeText))
				}
				cells = append(cells, deltaCell, changeCell)
			}
			rows = append(rows, cells)
		}
		writeTable(w, rows)
	}

	perfTable("Hardware Performance", telemetryMetricLabels, aibom.ComparePerformance(items, telemetryMetricOrder))

	// Only shown when some run carries inference telemetry; otherwise every row
	// is "—" and the table is noise for non-serving workloads.
	if anyInferenceMetrics(items) {
		perfTable("Inference Performance", servingMetricLabels, aibom.CompareInferencePerformance(items, servingMetricOrder))
	}
}

// anyInferenceMetrics reports whether at least one run has serving-engine
// telemetry (an inference.performance.metrics map).
func anyInferenceMetrics(items []aibom.AIBOM) bool {
	for _, a := range items {
		if a.Data.Inference != nil && a.Data.Inference.Performance != nil && len(a.Data.Inference.Performance.Metrics) > 0 {
			return true
		}
	}
	return false
}
