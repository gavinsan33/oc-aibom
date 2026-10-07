# oc aibom list

Lists AIBOMs with the fields you'd otherwise have to dig for in `-o yaml`: job, model, experiment intent, quantization, GPU type, and collection time.

```sh
oc aibom list -n my-namespace
oc aibom list -A                            # all namespaces
oc aibom list --model=granite-3.0-8b        # filter by model.name
oc aibom list --intent=sft                  # training | sft | inference
oc aibom list --quantization=int4
oc aibom list --architecture=llama          # filter by model.architecture
oc aibom list --framework=vllm              # filter by model.framework
oc aibom list --gpu-type=A100               # filter by environment.gpu_type
oc aibom list --job=my-training-job         # filter by job name
oc aibom list --git-branch=main             # filter by source_code.git_branch
oc aibom list --git-repository=my-org/repo  # filter by source_code.git_repository
oc aibom list --serving-engine=vllm         # filter by inference.serving_engine
oc aibom list --adaptation-method=lora      # filter by fine_tuning.adaptation_method
oc aibom list --optimizer=adamw             # filter by training.optimizer
oc aibom list --drift-only                  # auto-detected dataset != declared dataset
oc aibom list --sort-by=gpu-utilization     # rank by a performance metric (highest first)
oc aibom list --sort-by=gpu-power --ascending
```

## Filtering

All filters are exact-match and case-insensitive, and can be combined. Combined filters are AND'd together.

## Sorting

`--sort-by` accepts `gpu-utilization`, `gpu-memory`, `gpu-power`, `cpu-usage`, `memory-usage`, `network-rx` and `network-tx`. It adds the corresponding column to the table. Add `--ascending` to put the lowest value first.
