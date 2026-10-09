package main

import (
	"fmt"
	"strings"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

// environmentExtras returns the optional discovery lines (label, value) for
// `describe`, skipping anything absent so an older AIBOM prints nothing extra.
func environmentExtras(e aibom.Environment) [][2]string {
	var out [][2]string
	add := func(label string, parts ...string) {
		var kept []string
		for _, p := range parts {
			if p != "" {
				kept = append(kept, p)
			}
		}
		if len(kept) > 0 {
			out = append(out, [2]string{label, strings.Join(kept, ", ")})
		}
	}
	get := func(m map[string]any, k string) string {
		if v, ok := m[k]; ok {
			return strings.Join(strings.Fields(strings.ReplaceAll(fmt.Sprint(v), "\n", ", ")), " ")
		}
		return ""
	}
	labeled := func(label, v string) string {
		if v == "" {
			return ""
		}
		return label + " " + v
	}

	add("GPU Memory", formatGPUMemory(e.GPUMemoryMB))
	add("CPU Details", get(e.CPU, "cpu_architecture"),
		labeled("cores/socket", get(e.CPU, "cpu_cores_per_socket")),
		labeled("threads/core", get(e.CPU, "cpu_threads_per_core")),
		labeled("L3", get(e.CPU, "cache_l3")))
	add("Network", labeled("RDMA devices", get(e.Network, "rdma_devices")), labeled("MTU", get(e.Network, "primary_mtu")))
	add("Block Devices", get(e.Storage, "block_devices"))
	add("Kernel Config", labeled("governor", get(e.KernelConfig, "cpu_governor")),
		labeled("NUMA balancing", get(e.KernelConfig, "numa_balancing")),
		labeled("THP", get(e.KernelConfig, "transparent_hugepages")),
		labeled("max_map_count", get(e.KernelConfig, "max_map_count")))
	return out
}

// formatGPUMemory renders per-GPU MiB as GiB, collapsing identical GPUs
// ("2 x 80 GiB"). Empty for no data.
func formatGPUMemory(mb []aibom.FlexInt) string {
	if len(mb) == 0 {
		return ""
	}
	gib := func(m aibom.FlexInt) string { return fmt.Sprintf("%.0f GiB", float64(m)/1024) }
	same := true
	for _, m := range mb {
		same = same && m == mb[0]
	}
	if same {
		return fmt.Sprintf("%d x %s", len(mb), gib(mb[0]))
	}
	parts := make([]string, len(mb))
	for i, m := range mb {
		parts[i] = gib(m)
	}
	return strings.Join(parts, ", ")
}
