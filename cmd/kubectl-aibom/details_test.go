package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func TestPrintEnvironmentDetails(t *testing.T) {
	var buf bytes.Buffer
	printEnvironmentDetails(&buf, aibom.Environment{
		GPUMemoryMB: []aibom.FlexInt{81920, 81920},
		CPU:         map[string]any{"cpu_architecture": "x86_64", "cache_l3": "24 MiB", "future_key": "v"},
		Storage:     map[string]any{"block_devices": "nvme0n1 894G\nsda 1T"},
		Benchmarks:  map[string]any{"cpu_compute": map[string]any{"mflops": "100.00", "time_seconds": "1.2"}},
	})
	out := buf.String()
	for _, want := range []string{
		"2 x 80 GiB", "Architecture", "x86_64", "L3 cache", "Future key",
		"nvme0n1 894G, sda 1T", "Benchmark", "Cpu compute", "Mflops", "100.00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Network:") {
		t.Errorf("empty sections must be omitted:\n%s", out)
	}
	var empty bytes.Buffer
	printEnvironmentDetails(&empty, aibom.Environment{})
	if empty.Len() != 0 {
		t.Errorf("an AIBOM without these fields should print nothing:\n%s", empty.String())
	}
}
