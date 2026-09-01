# oc-aibom

A `kubectl`/`oc` plugin that makes it easier to filter through and compare
[AIBOM](https://github.com/gsanders/aibom-webhook-service) (`aibom.io/v1alpha1`)
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
oc aibom list --drift-only                  # auto-detected dataset != declared dataset
oc aibom list --sort-by=gpu-utilization     # rank by a performance metric (highest first)
oc aibom list --sort-by=gpu-power --ascending
```

`--sort-by` accepts: `gpu-utilization`, `gpu-memory`, `gpu-power`,
`cpu-usage`, `memory-usage`, `network-rx`, `network-tx` — and adds the
corresponding column to the table.

### `oc aibom describe <name>`

Prints a human-readable summary of a single AIBOM's model, dataset
(declared vs. auto-detected, flagging mismatches), source provenance,
environment, and resource utilization (GPU/CPU/memory/network averages,
plus any Grafana links) — instead of a raw YAML dump.

### `oc aibom diff <name-a> <name-b>`

Field-by-field comparison of two AIBOMs: model config, dataset
declaration/drift, git provenance, and hardware/driver environment, plus a
quantified performance table (value, delta, and percent change) across GPU
utilization/memory/power, CPU/memory usage, and network throughput.

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
- `make install` — build and install to `/usr/local/bin` (or `INSTALL_DIR`)
- `make uninstall` — remove the installed binary from `INSTALL_DIR`
- `make dist` — cross-compile release tarballs + `checksums.txt` into `dist/` for krew
- `make test` — run unit tests
- `make vet` — run `go vet`
- `make check` — `vet` + `test`
- `make fmt` — `gofmt` the repo
- `make tidy` — `go mod tidy`
- `make clean` — remove the built binary and `dist/`

## Project structure

```
cmd/kubectl-aibom/   CLI entrypoint (cobra commands, table/summary output)
internal/aibom/      AIBOM types, dynamic-client queries, filter/diff logic
```
