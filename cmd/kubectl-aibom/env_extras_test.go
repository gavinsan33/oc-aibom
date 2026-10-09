package main

import (
	"reflect"
	"testing"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

func TestEnvironmentExtras(t *testing.T) {
	got := environmentExtras(aibom.Environment{
		GPUMemoryMB: []aibom.FlexInt{81920, 81920},
		CPU:         map[string]any{"cpu_architecture": "x86_64", "cpu_cores_per_socket": "8", "cache_l3": "24 MiB"},
		Network:     map[string]any{"primary_mtu": "1400"},
		Storage:     map[string]any{"block_devices": "nvme0n1 894G\nsda 1T"},
	})
	want := [][2]string{
		{"GPU Memory", "2 x 80 GiB"},
		{"CPU Details", "x86_64, cores/socket 8, L3 24 MiB"},
		{"Network", "MTU 1400"},
		{"Block Devices", "nvme0n1 894G, sda 1T"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if n := len(environmentExtras(aibom.Environment{})); n != 0 {
		t.Fatalf("an AIBOM without these fields should print nothing extra, got %d lines", n)
	}
}
