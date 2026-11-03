package aibom

import (
	"fmt"
	"strings"
)

// The summaries below are shared by `describe` (which adds where each was
// found) and `compare`/`diff` (which don't, so two runs differing only in
// the source aren't reported as different). All return "" for a nil block.

// Parallelism is the strategies joined, e.g. "fsdp + tensor_parallel".
func (d *Distributed) Parallelism() string {
	if d == nil {
		return ""
	}
	return strings.Join(d.Strategies, " + ")
}

// Topology is e.g. "8 processes, 2 nodes, 4 per node".
func (d *Distributed) Topology() string {
	if d == nil {
		return ""
	}
	parts := []string{}
	if d.WorldSize != nil {
		parts = append(parts, plural(*d.WorldSize, "process", "processes"))
	}
	if d.Nodes != nil {
		parts = append(parts, plural(*d.Nodes, "node", "nodes"))
	}
	if d.ProcessesPerNode != nil {
		parts = append(parts, fmt.Sprintf("%d per node", *d.ProcessesPerNode))
	}
	return strings.Join(parts, ", ")
}

// ParallelSizes is e.g. "tensor 2, data 4".
func (d *Distributed) ParallelSizes() string {
	if d == nil {
		return ""
	}
	parts := []string{}
	for _, s := range []struct {
		name string
		v    *int
	}{{"tensor", d.TensorParallelSize}, {"pipeline", d.PipelineParallelSize}, {"context", d.ContextParallelSize},
		{"expert", d.ExpertParallelSize}, {"data", d.DataParallelSize}} {
		if s.v != nil {
			parts = append(parts, fmt.Sprintf("%s %d", s.name, *s.v))
		}
	}
	return strings.Join(parts, ", ")
}

// FSDPSummary is e.g. "FSDP2, reshard after forward true, CPU offload false",
// or "enabled" when FSDP was seen with no detail.
func (d *Distributed) FSDPSummary() string {
	if d == nil || d.FSDP == nil {
		return ""
	}
	f := d.FSDP
	parts := []string{}
	if f.Version != nil {
		parts = append(parts, fmt.Sprintf("FSDP%d", *f.Version))
	}
	if f.ShardingStrategy != "" {
		parts = append(parts, f.ShardingStrategy)
	}
	if f.ReshardAfterForward != nil {
		parts = append(parts, fmt.Sprintf("reshard after forward %v", *f.ReshardAfterForward))
	}
	if f.CPUOffload != nil {
		parts = append(parts, fmt.Sprintf("CPU offload %v", *f.CPUOffload))
	}
	return orEnabled(parts)
}

// DeepSpeedSummary is e.g. "ZeRO-3, optimizer offload cpu", or "enabled".
func (d *Distributed) DeepSpeedSummary() string {
	if d == nil || d.DeepSpeed == nil {
		return ""
	}
	z := d.DeepSpeed
	parts := []string{}
	if z.ZeroStage != nil {
		parts = append(parts, fmt.Sprintf("ZeRO-%d", *z.ZeroStage))
	}
	if z.OffloadOptimizer != "" {
		parts = append(parts, "optimizer offload "+z.OffloadOptimizer)
	}
	if z.OffloadParam != "" {
		parts = append(parts, "param offload "+z.OffloadParam)
	}
	return orEnabled(parts)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func orEnabled(parts []string) string {
	if len(parts) == 0 {
		return "enabled"
	}
	return strings.Join(parts, ", ")
}
