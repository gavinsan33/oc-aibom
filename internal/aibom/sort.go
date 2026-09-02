package aibom

import (
	"fmt"
	"sort"
)

// SortableMetrics lists the --sort-by keys accepted by `list`, mapped to
// their resource_utilization accessor.
var SortableMetrics = map[string]func(ResourceUtilization) float64{
	"gpu-utilization": func(r ResourceUtilization) float64 { return r.MetricAvg("gpu_utilization") },
	"gpu-memory":      func(r ResourceUtilization) float64 { return r.MetricAvg("gpu_memory_used") },
	"gpu-power":       func(r ResourceUtilization) float64 { return r.MetricAvg("gpu_power") },
	"cpu-usage":       func(r ResourceUtilization) float64 { return r.MetricAvg("cpu_usage") },
	"memory-usage":    func(r ResourceUtilization) float64 { return r.MetricAvg("memory_usage") },
	"network-rx":      func(r ResourceUtilization) float64 { return r.MetricAvg("network_receive") },
	"network-tx":      func(r ResourceUtilization) float64 { return r.MetricAvg("network_transmit") },
}

// SortByMetric sorts items in place by the named performance metric,
// descending (highest first) unless ascending is set. Returns an error if
// metric isn't a recognized SortableMetrics key.
func SortByMetric(items []AIBOM, metric string, ascending bool) error {
	get, ok := SortableMetrics[metric]
	if !ok {
		return fmt.Errorf("unknown --sort-by metric %q (want one of: gpu-utilization, gpu-memory, gpu-power, cpu-usage, memory-usage, network-rx, network-tx)", metric)
	}
	sort.SliceStable(items, func(i, j int) bool {
		vi, vj := get(items[i].Data.ResourceUtilization), get(items[j].Data.ResourceUtilization)
		if ascending {
			return vi < vj
		}
		return vi > vj
	})
	return nil
}
