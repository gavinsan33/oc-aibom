package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gsanders/oc-aibom/internal/aibom"
)

// A training block as aibom-webhook-service's postprocess writes it for a
// Trainer run with weight decay (two param groups) and a reseeded torch RNG.
const observedTraining = `{
  "optimizer": "adamw_torch_fused",
  "optimizer_config": {
    "class": "torch.optim.adamw.AdamW",
    "params": {"lr": 0.0003, "betas": [0.9, 0.95], "eps": 1e-08, "fused": true},
    "param_groups": [{"weight_decay": 0.1, "param_tensors": 6}, {"weight_decay": 0.0, "param_tensors": 10}],
    "declared_via": "runtime_optimizer"
  },
  "lr_scheduler": {"type": "cosine", "warmup_steps": 0.05, "declared_via": "training_arguments"},
  "mixed_precision": "bf16",
  "determinism": {
    "seeds": {"torch": 1234, "python": 42, "numpy": 42},
    "seed_history": {"torch": [42, 1234]},
    "data_seed": 7,
    "full_determinism": true,
    "cudnn_deterministic": true,
    "cudnn_benchmark": false,
    "deterministic_algorithms": true,
    "deterministic_algorithms_warn_only": true,
    "cublas_workspace_config": ":4096:8",
    "python_hash_seed": "0"
  }
}`

func decodeTraining(t *testing.T, raw string) aibom.Training {
	t.Helper()
	var tr aibom.Training
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestTrainingExtrasObservedOptimizer(t *testing.T) {
	got := trainingExtras(decodeTraining(t, observedTraining))
	want := [][2]string{
		{"Optimizer Class", "torch.optim.adamw.AdamW (via: runtime_optimizer)"},
		{"Optimizer Params", "betas=[0.9, 0.95], eps=1e-08, fused=true, lr=0.0003"},
		{"Param Groups", "weight_decay=0.1 (6 tensors); weight_decay=0 (10 tensors)"},
		{"LR Scheduler", "cosine, warmup steps 0.05 (via: training_arguments)"},
		{"Mixed Precision", "bf16"},
		{"Seeds", "python=42, numpy=42, torch=1234"},
		{"Seed History", "torch: 42 → 1234"},
		{"Data Seed", "7"},
		{"Determinism", "cudnn deterministic true, cudnn benchmark false, deterministic algorithms true (warn only), full determinism"},
		{"CUBLAS Workspace", ":4096:8"},
		{"PYTHONHASHSEED", "0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%v\nwant\n%v", got, want)
	}
}

func TestTrainingExtrasDeclaredOptimizer(t *testing.T) {
	got := trainingExtras(decodeTraining(t, `{
	  "optimizer_config": {"params": {"betas": [0.9, 0.999], "weight_decay": 0.05}, "optim_args": "foo=1", "declared_via": "cli_arg"},
	  "determinism": {"seeds": {"python": "abc", "torch": "18446744073709551616"}}
	}`))
	want := [][2]string{
		// No class to hang `via` on, so it goes on the params.
		{"Optimizer Params", "betas=[0.9, 0.999], weight_decay=0.05 (via: cli_arg)"},
		{"Optim Args", "foo=1"},
		{"Seeds", "python=abc, torch=18446744073709551616"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%v\nwant\n%v", got, want)
	}
}

func TestTrainingExtrasOlderAIBOM(t *testing.T) {
	tr := decodeTraining(t, `{"optimizer": "adamw_torch", "learning_rate": 0.0002, "determinism": null}`)
	if n := len(trainingExtras(tr)); n != 0 {
		t.Fatalf("an AIBOM without these fields should print nothing extra, got %d lines", n)
	}
}

func TestFormatValue(t *testing.T) {
	for v, want := range map[any]string{2e-4: "0.0002", 42.0: "42", 1e-8: "1e-08", true: "true", "x": "x"} {
		if got := formatValue(v); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", v, got, want)
		}
	}
	if got := formatValue(nil); got != "null" {
		t.Errorf("formatValue(nil) = %q", got)
	}
}

func TestDescribePrintsTrainingExtras(t *testing.T) {
	out := describeOutput(t, `{"training": `+observedTraining+`}`)
	for _, line := range []string{
		"  Optimizer:              adamw_torch_fused\n",
		"  Optimizer Class:        torch.optim.adamw.AdamW (via: runtime_optimizer)\n",
		"  Seeds:                  python=42, numpy=42, torch=1234\n",
	} {
		if !strings.Contains(out, line) {
			t.Fatalf("missing %q in:\n%s", line, out)
		}
	}
}

func TestTrainingExtrasPrecision(t *testing.T) {
	got := trainingExtras(decodeTraining(t, `{
	  "mixed_precision": "bf16",
	  "mixed_precision_declared_via": "accelerator",
	  "precision": {
	    "tf32_matmul": true, "tf32_cudnn": false, "float32_matmul_precision": "high",
	    "autocast": ["cuda:bfloat16", "cpu:bfloat16"],
	    "fp8": {"backend": "transformer_engine", "recipe": "DelayedScaling"},
	    "fsdp_mixed_precision": {"policy": "MixedPrecision", "param_dtype": "bfloat16", "reduce_dtype": "float32", "buffer_dtype": "bfloat16"},
	    "deepspeed": "no"
	  }
	}`))
	want := [][2]string{
		{"Mixed Precision", "bf16 (via: accelerator)"},
		{"TF32", "matmul true, cudnn false, float32 matmul precision high"},
		{"Autocast", "cuda:bfloat16, cpu:bfloat16"},
		{"FP8", "transformer_engine (recipe DelayedScaling)"},
		{"FSDP Precision", "MixedPrecision: param bfloat16, reduce float32, buffer bfloat16"},
		{"DeepSpeed Precision", "no"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTrainingExtrasFP8WithoutBackend(t *testing.T) {
	got := trainingExtras(decodeTraining(t, `{"precision": {"fp8": {"backend": null}}}`))
	want := [][2]string{{"FP8", "unknown backend"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTrainingExtrasDistributed(t *testing.T) {
	got := trainingExtras(decodeTraining(t, `{"distributed": {
	  "world_size": 8, "nodes": 2, "processes_per_node": 4, "world_declared_via": "environment",
	  "tensor_parallel_size": 2, "data_parallel_size": 4, "parallel_sizes_declared_via": "device_mesh",
	  "device_mesh": {"dp_shard": 4, "tp": 2},
	  "fsdp": {"version": 2, "reshard_after_forward": true, "cpu_offload": false, "declared_via": "fsdp_module"},
	  "deepspeed": {"zero_stage": 3, "offload_optimizer": "cpu", "declared_via": "deepspeed_engine"},
	  "strategies": ["fsdp", "tensor_parallel"]
	}}`))
	want := [][2]string{
		{"Topology", "8 processes, 2 nodes, 4 per node (via: environment)"},
		{"Parallel Sizes", "tensor 2, data 4 (via: device_mesh)"},
		{"Device Mesh", "dp_shard=4, tp=2"},
		{"FSDP", "FSDP2, reshard after forward true, CPU offload false (via: fsdp_module)"},
		{"DeepSpeed", "ZeRO-3, optimizer offload cpu (via: deepspeed_engine)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTrainingExtrasDistributedEmptyBlocks(t *testing.T) {
	got := trainingExtras(decodeTraining(t, `{"distributed": {"fsdp": {"declared_via": "accelerate_config"}, "strategies": ["fsdp"]}}`))
	want := [][2]string{{"FSDP", "enabled (via: accelerate_config)"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
