# oc-aibom

A `kubectl`/`oc` plugin that makes it easier to filter through and compare
[AIBOM](https://github.com/gavinsan33/aibom-webhook-service) (`aibom.io/v1alpha1`)
custom resources than raw `oc get aibom -o yaml`, using only the standard
Kubernetes API.

Examples below use `oc`, but since `oc` is a superset of `kubectl` and both
use the same plugin mechanism, every command works identically with
`kubectl` too — swap one for the other freely.

## Install

Preferred: via [krew](https://krew.sigs.k8s.io/), kubectl's plugin manager,
using this repo as a self-hosted plugin index:

```sh
oc krew index add oc-aibom https://github.com/gavinsan33/oc-aibom.git
oc krew install oc-aibom/aibom
```

Or build from source (requires Go 1.22+):

```sh
make install   # builds and installs to /usr/local/bin (override with INSTALL_DIR=...)
```

Either way, invoke it as a plugin:

```sh
oc aibom list
oc aibom describe <name>
oc aibom diff <name-a> <name-b>
oc aibom compare <name> <name> [<name>...]
```

### Shell completion

`kubectl`/`oc` don't call a plugin's own `__complete` command directly —
they look on `PATH` for a companion executable named `kubectl_complete-aibom`
(or `oc_complete-aibom`), and silently fall back to filename completion if
it's missing. `make install` installs both companion scripts (from
`completion/`) alongside the binary, so `oc aibom <TAB>` and
`oc aibom list --<TAB>` work out of the box for source installs, as long as
your shell has `oc`'s (or `kubectl`'s) own completion sourced, e.g.:

```sh
source <(oc completion zsh)   # or: source <(oc completion bash)
```

krew installs don't get this automatically — krew only symlinks the
manifest's `bin:` entry onto `PATH`, not the completion scripts bundled in
the release tarball. To enable completion for a krew install, copy the
extracted `kubectl_complete-aibom`/`oc_complete-aibom` files (from
`~/.krew/store/aibom/<version>/`) into `~/.krew/bin` yourself.

## Usage

### `oc aibom list`

Lists AIBOMs with the fields you'd otherwise have to dig for in `-o yaml`:
job, model, experiment intent, quantization, GPU type, collection time.

```
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

All filters are exact-match, case-insensitive, and can be combined (AND'd
together). `--sort-by` accepts: `gpu-utilization`, `gpu-memory`, `gpu-power`,
`cpu-usage`, `memory-usage`, `network-rx`, `network-tx` — and adds the
corresponding column to the table.

### `oc aibom describe <name>`

Prints a human-readable summary of a single AIBOM's model, dataset
(declared vs. auto-detected, flagging mismatches), source provenance,
environment, and resource utilization (GPU/CPU/memory/network averages,
plus any Grafana links) — instead of a raw YAML dump. If the AIBOM was
compiled with per-metric detail (aibom-webhook-service's segmented telemetry
stats — see its `CLAUDE.md`), a "Performance Detail" section also shows each
metric's min/avg/max/p95 and its first→middle→last-third breakdown across
the run, with a trend arrow (↑/↓/→) flagging runs that ramped up, throttled
down, or held steady — something a single run-wide average can't show.

For an inference AIBOM whose serving engine has its own telemetry (vLLM
today — see aibom-webhook-service's CLAUDE.md, "Inference Performance
Telemetry"), an "Inference Performance" section shows vLLM's own
serving-level metrics (TTFT, ITL, queue depth, KV-cache usage, throughput)
with the same min/avg/max/p95/segments detail — a distinct section from
resource utilization, since these describe the serving application's own
behavior rather than the hardware underneath it. Absent on an AIBOM
predating this field, or one whose serving engine isn't covered yet.

For a KServe model pre-pulled onto a PVC, the Model section also shows how
the name was determined (`via: model_files_readme` means it came from the
model's own README rather than the PVC folder name), plus the pinned
`Revision`, `Base Model`, and `Size` read from the model's files. This is
identification, not verification: nothing checks the files against the
weights. `diff` flags a changed `model.revision`.

A `Signature:` line reports whether the AIBOM's Ed25519 signature (see
aibom-webhook-service's `CLAUDE.md`, "Compiled AIBOM Signing") checks out:

- **`✓ Verified`** (green) — the signature matches the AIBOM's data, *and*
  the embedded public key matches the one the cluster currently publishes.
  The only status that should be read as "trust this."
- **`not signed`** — no signature present, e.g. an AIBOM created before the
  signing feature shipped, or in a namespace with no signing key configured.
  Not itself a bad sign.
- **`signed (unconfirmed)`** (yellow) — the signature is internally
  consistent, but the cluster's published key couldn't be checked (no RBAC,
  no network, or the ConfigMap doesn't exist yet). Weaker than `Verified`:
  it only proves the signature matches *some* key, not that the key is the
  cluster's real one.
- **`⚠ KEY MISMATCH`** (yellow) — the signature is valid, but the embedded
  key doesn't match the cluster's current published key. Can follow a
  legitimate key rotation, or indicate the embedded key was forged.
- **`✗ INVALID SIGNATURE`** (red) — the signature does not match the data.
  This is the tamper/corruption case.

When the AIBOM has a persisted telemetry time series (its signed data carries
a `telemetry_series_ref` pointing at an `AIBOMTelemetry` object — see
aibom-webhook-service's `CLAUDE.md`, "Telemetry Time Series"), a
`Telemetry Series:` line reports whether that object's `spec.seriesJson`
still hashes to the sha256 recorded in the signed data. AIBOMs without a
stored series print no such line.

- **`✓ Verified`** (green) — the content matches the digest, *and* the AIBOM's
  own signature is `✓ Verified`, so the digest itself is authenticated.
- **`digest matches (AIBOM not verified)`** (yellow) — the content matches, but
  the AIBOM's signature isn't verified (unsigned, unconfirmed, ...). The digest
  lives inside the AIBOM's data, so this only shows the series wasn't changed
  independently of the AIBOM, not that either is genuine.
- **`✗ DIGEST MISMATCH`** (red) — the object's content (or size) no longer
  matches the signed digest: it was altered or replaced after the fact.
- **`missing`** (yellow) — the referenced object no longer exists.
- **`unconfirmed`** (yellow) — it couldn't be checked (no RBAC to read
  `aibomtelemetries`, no cluster, or a reference shape this version doesn't
  understand, such as the short-lived ConfigMap form).

### `oc aibom graph <name> [<name>...]`

Charts AIBOMs' stored telemetry time series (the `AIBOMTelemetry` object each
`telemetry_series_ref` points at — see aibom-webhook-service's `CLAUDE.md`,
"Telemetry Time Series"), so it works after Prometheus's retention window has
passed. On a terminal it opens a full-screen, btop-style view (alternate
screen, like `less`): a grid of braille line charts, one per metric, with
every given run overlaid in its own color. Like the console plugin's compare
telemetry tab, x is elapsed time since each run's own start on a shared axis
(a shorter run is a shorter line) and the y scale is shared across runs.

| Key | Action |
| --- | --- |
| `←` `→` `↑` `↓` / `hjkl` / `tab` | select a metric |
| `enter` / `z` | zoom the selected metric full screen (`enter`, `q` or `esc` to go back) |
| `1`–`9` | hide / show that run (the y scale stays put) |
| `p` | per-pod / GPU / container lines instead of each run's aggregate |
| `q` / `esc` | quit (when zoomed, go back first; `ctrl+c` always quits) |

Where runs' lines coincide the cell is drawn white, since a terminal cell
can only hold one color; hide runs with `1`–`9` to see each one on its own.

The footer shows the selected metric's min/avg/max per run (and per-bucket
peak where recorded). Series that fail the digest check are left out, as are
runs with no stored series; the verification result for each run is printed
when you quit.

When stdout isn't a terminal (or with `--text`) it prints one sparkline row
per run per metric instead, which is handy in logs and pipes.

```
oc aibom graph my-run
oc aibom graph run1 run2 run3 --metric gpu_utilization,memory_usage
oc aibom graph run1 run2 --pods
oc aibom graph run1 run2 --text --width 100
```

### `oc aibom diff <name-a> <name-b>`

Field-by-field comparison of two AIBOMs: model config, dataset
declaration/drift, git provenance, and hardware/driver environment, plus a
quantified performance table (value, delta, and percent change) across GPU
utilization/memory/power, CPU/memory usage, and network throughput. When
segmented telemetry stats are available, each run's own value is annotated
with its within-run trend arrow — distinct from the delta/change columns,
which only compare the two runs' averages against each other.

### `oc aibom compare <name> <name> [<name>...]`

Side-by-side performance table across two or more AIBOMs — one column per
run — for spotting trends across a run history rather than just a single
pairwise diff.

## Standard flags

Built with `k8s.io/cli-runtime`'s `genericclioptions`, so it accepts the
same connection flags `oc`/`kubectl` themselves do: `--kubeconfig`,
`--context`, `--namespace`/`-n`, `--server`, `--token`, etc. — no
plugin-specific config needed.


## Implementation notes

There is no generated Go clientset for the `AIBOM` custom resource (it's
created directly from Python in aibom-webhook-service), so this plugin
talks to the API via `k8s.io/client-go`'s dynamic client against
`aibom.io/v1alpha1, Resource: aiboms`, and decodes `spec.data` into Go
structs mirroring `compile_aibom()`'s output for typed filtering/diffing.

## Releasing (krew)

Pushing a `v*` tag runs `.github/workflows/release.yaml`, which builds
cross-platform archives with `make dist` and publishes them as a GitHub
release. `plugins/aibom.yaml` is the krew manifest for this repo's index —
after cutting a release, update its `sha256` fields from the **release's own
`checksums.txt` asset** (`gh release download <tag> -p checksums.txt`), not
from a local `make dist` rebuild — Go builds aren't bit-for-bit reproducible
across different checkout paths, so a locally-built tarball's hash won't
match the one CI actually published.

To test the manifest locally before/without a release:

```sh
make dist VERSION=v0.0.0-dev
kubectl krew install --manifest=plugins/aibom.yaml --archive=dist/kubectl-aibom_v0.0.0-dev_linux_amd64.tar.gz
```

## Makefile targets

- `make build` — build `./kubectl-aibom`
- `make install` — build and install the binary + shell-completion companion scripts to `/usr/local/bin` (or `INSTALL_DIR`)
- `make uninstall` — remove the installed binary + completion scripts from `INSTALL_DIR`
- `make dist` — cross-compile release tarballs (binary + completion scripts) + `checksums.txt` into `dist/` for krew
- `make test` — run unit tests
- `make vet` — run `go vet`
- `make check` — `vet` + `test`
- `make fmt` — `gofmt` the repo
- `make tidy` — `go mod tidy`
- `make clean` — remove the built binary and `dist/`

## Project structure

```
cmd/kubectl-aibom/   CLI entrypoint (cobra commands, table/summary output, graph TUI)
internal/aibom/      AIBOM types, dynamic-client queries, filter/diff logic
```
