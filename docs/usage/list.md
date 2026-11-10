# oc aibomrun list

Lists AIBOMs with the fields you'd otherwise have to dig for in `-o yaml`: job, model, experiment intent, quantization, GPU type, and collection time.

```sh
oc aibomrun list -n my-namespace
oc aibomrun list -A                            # all namespaces
oc aibomrun list --model=granite-3.0-8b        # filter by model.name
oc aibomrun list --intent=sft                  # training | sft | inference
oc aibomrun list --quantization=int4
oc aibomrun list --architecture=llama          # filter by model.architecture
oc aibomrun list --framework=vllm              # filter by model.framework
oc aibomrun list --gpu-type=A100               # filter by environment.gpu_type
oc aibomrun list --job=my-training-job         # filter by job name
oc aibomrun list --git-branch=main             # filter by source_code.git_branch
oc aibomrun list --git-repository=my-org/repo  # filter by source_code.git_repository
oc aibomrun list --serving-engine=vllm         # filter by inference.serving_engine
oc aibomrun list --adaptation-method=lora      # filter by fine_tuning.adaptation_method
oc aibomrun list --optimizer=adamw             # filter by training.optimizer
oc aibomrun list --drift-only                  # auto-detected dataset != declared dataset
oc aibomrun list --sort-by=gpu-utilization     # rank by a performance metric (highest first)
oc aibomrun list --sort-by=gpu-power --ascending
```

## Interactive browser

Run `oc aibomrun` with no subcommand on a terminal (stdout not piped) and it opens a full-screen browser over the same table: move a cursor through the rows and drill into a single AIBOM's `describe`, a side-by-side `compare` of several, or the overlaid `graph` view — without leaving the terminal. Piping (`oc aibomrun | grep …`) still prints the plain table. Any `--sort-by`/`--ascending`/filter flags you pass seed the browser's initial state.

### Keys

| Key | Action |
|-----|--------|
| `↑` `↓`, `k` `j` | Move the cursor (`home`/`end`, `G` jump to ends) |
| `←` `→`, `h` `l` | Scroll a wide table sideways |
| `space` | Select / deselect the row |
| `enter` | Describe the row, or compare when two or more are selected |
| `g` | Graph the selection (or the row under the cursor) |
| `s` | Open the **sort** menu (pick a field and direction) |
| `f`, `/` | Open the live **filter** prompt |
| `esc` | Clear the selection |
| `q`, `ctrl+c` | Quit |

In the **sort** menu: `↑`/`↓` pick a field (age plus the `--sort-by` metrics), `space` or `←`/`→` flips the direction, `enter` applies, `esc` cancels.

The sort you last applied from this menu is remembered (in `~/.config/oc-aibom/prefs.json`, or your OS's user config directory) and restored the next time the browser opens, unless you pass `--sort-by` or `--ascending`. The plain `list` output always follows its flags.

In the **filter** prompt: type to narrow the list live (a case-insensitive substring over the visible columns), `enter` keeps the filter, `esc` clears it.

Inside the describe/compare/graph pages, a `[d] Detail view` / `[g] Graph view` tab bar shows where you are; `d` and `g` switch between the two views of the same subject, and `q`/`esc` backs out to the list.

## Filtering

All filters are exact-match and case-insensitive, and can be combined. Combined filters are AND'd together.

## Sorting

`--sort-by` accepts `gpu-utilization`, `gpu-memory`, `gpu-power`, `cpu-usage`, `memory-usage`, `network-rx` and `network-tx`. It adds the corresponding column to the table. Add `--ascending` to put the lowest value first.
