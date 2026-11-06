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

func TestEnvironmentExtrasSoftwareVersions(t *testing.T) {
	got := environmentExtras(aibom.Environment{
		PythonVersion:    "3.12.16",
		TorchCUDAVersion: "13.0",
		CuDNNVersion:     "9.24.0",
		NCCLVersion:      "2.30.7",
		Libraries:        map[string]string{"torch": "2.14.1+cu130", "sglang": "0.5.23", "accelerate": "1.15.0"},
	})
	want := [][2]string{
		{"Python", "3.12.16"},
		{"Torch Build", "CUDA 13.0, cuDNN 9.24.0, NCCL 2.30.7"},
		{"Libraries", "accelerate 1.15.0, sglang 0.5.23, torch 2.14.1+cu130"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
