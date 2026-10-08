package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

// detailField maps a discovery key to a display label. Keys listed here
// print first, in order; any other key a newer webhook adds still prints
// afterwards under a humanized name rather than being dropped.
type detailField struct{ key, label string }

type detailGroup struct {
	title  string
	fields []detailField
	data   map[string]any
}

// printEnvironmentDetails renders the optional discovery sections shown by
// `describe --detailed`: one titled block per section with aligned
// label/value rows, and the benchmarks as a table. Silent for an AIBOM
// predating these fields.
func printEnvironmentDetails(w io.Writer, e aibom.Environment) {
	groups := []detailGroup{
		{"CPU", []detailField{
			{"cpu_architecture", "Architecture"}, {"cpu_cores_per_socket", "Cores per socket"},
			{"cpu_threads_per_core", "Threads per core"}, {"cpu_min_freq_mhz", "Min clock (MHz)"},
			{"cpu_max_freq_mhz", "Max clock (MHz)"}, {"cache_l1d", "L1d cache"}, {"cache_l1i", "L1i cache"},
			{"cache_l2", "L2 cache"}, {"cache_l3", "L3 cache"},
		}, e.CPU},
		{"Network", []detailField{
			{"interface_names", "Interfaces"}, {"primary_mtu", "Primary MTU"},
			{"rdma_devices", "RDMA devices"}, {"rdma_device_count", "RDMA device count"},
			{"tcp_congestion_control", "TCP congestion control"}, {"tcp_rmem", "TCP read buffers"},
			{"tcp_wmem", "TCP write buffers"},
		}, e.Network},
		{"Storage", []detailField{
			{"block_devices", "Block devices"}, {"tmpfs_size", "/tmp size"}, {"tmpfs_avail", "/tmp available"},
		}, e.Storage},
		{"Kernel Config", []detailField{
			{"cpu_governor", "CPU governor"}, {"numa_balancing", "NUMA balancing"},
			{"transparent_hugepages", "Transparent hugepages"}, {"swappiness", "Swappiness"},
			{"dirty_ratio", "Dirty ratio"}, {"dirty_background_ratio", "Dirty background ratio"},
			{"max_map_count", "Max map count"}, {"file_max", "Max open files (system)"},
		}, e.KernelConfig},
		{"Process Limits", []detailField{
			{"max_user_processes", "Max user processes"}, {"max_open_files", "Max open files"},
			{"max_stack_size_kb", "Max stack size (KB)"}, {"max_memory_size_kb", "Max memory size (KB)"},
		}, e.ProcessLimits},
	}

	var body strings.Builder
	if len(e.GPUMemoryMB) > 0 {
		writeIndented(&body, [][]cell{{labelCell("GPU Memory"), plainCell(formatGPUMemory(e.GPUMemoryMB))}})
	}
	for _, g := range groups {
		if rows := detailRows(g); len(rows) > 0 {
			fmt.Fprintln(&body, " ", bold(g.title+":"))
			writeIndented(&body, rows)
		}
	}
	if rows := benchmarkRows(e.Benchmarks); len(rows) > 0 {
		fmt.Fprintln(&body, " ", bold("Benchmarks:"))
		writeIndented(&body, append([][]cell{headerRow("Benchmark", "Metric", "Value")}, rows...))
	}
	if body.Len() == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Hardware Details:"))
	fmt.Fprint(w, body.String())
}

// writeIndented prints a table nested two levels under its heading.
func writeIndented(w io.Writer, rows [][]cell) {
	var sb strings.Builder
	writeTable(&sb, rows)
	for _, line := range strings.Split(strings.TrimRight(sb.String(), "\n"), "\n") {
		fmt.Fprintln(w, "   ", line)
	}
}

func detailRows(g detailGroup) [][]cell {
	var rows [][]cell
	seen := map[string]bool{}
	add := func(key, label string) {
		if v, ok := g.data[key]; ok {
			rows = append(rows, []cell{labelCell(label), plainCell(formatDetail(v))})
		}
		seen[key] = true
	}
	for _, f := range g.fields {
		add(f.key, f.label)
	}
	var extra []string
	for k := range g.data {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		add(k, humanize(k))
	}
	return rows
}

// benchmarkRows flattens {benchmark: {metric: value}} into sorted rows.
func benchmarkRows(b map[string]any) [][]cell {
	names := make([]string, 0, len(b))
	for n := range b {
		names = append(names, n)
	}
	sort.Strings(names)
	var rows [][]cell
	for _, n := range names {
		metrics, ok := b[n].(map[string]any)
		if !ok {
			rows = append(rows, []cell{labelCell(humanize(n)), plainCell(""), plainCell(formatDetail(b[n]))})
			continue
		}
		keys := make([]string, 0, len(metrics))
		for k := range metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			name := ""
			if i == 0 {
				name = humanize(n)
			}
			rows = append(rows, []cell{labelCell(name), plainCell(humanize(k)), plainCell(formatDetail(metrics[k]))})
		}
	}
	return rows
}

// formatGPUMemory renders per-GPU MiB as GiB, collapsing identical GPUs
// ("2 x 80 GiB") the way they usually appear.
func formatGPUMemory(mb []aibom.FlexInt) string {
	same := true
	for _, m := range mb {
		same = same && m == mb[0]
	}
	gib := func(m aibom.FlexInt) string { return fmt.Sprintf("%.0f GiB", float64(m)/1024) }
	if same {
		return fmt.Sprintf("%d x %s", len(mb), gib(mb[0]))
	}
	parts := make([]string, len(mb))
	for i, m := range mb {
		parts[i] = gib(m)
	}
	return strings.Join(parts, ", ")
}

// formatDetail prints a discovery value; multi-line shell output (lsblk)
// becomes comma-separated.
func formatDetail(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", ", ")), " ")
}

func humanize(key string) string {
	s := strings.ReplaceAll(key, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
