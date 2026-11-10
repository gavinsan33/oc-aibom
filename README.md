<p align="center">
  <img src=".github/logo.svg" alt="" width="128">
</p>

<h1 align="center">oc-aibom</h1>

<p align="center"><strong>List, inspect, diff and compare AIBOMs from the terminal</strong></p>

<p align="center">
  <a href="https://gavinsan33.github.io/project-aibom/oc-aibom/"><img src="https://img.shields.io/badge/docs-Project%20AIBOM-00897b?logo=materialformkdocs&logoColor=white" alt="Documentation"></a>
  <a href="https://github.com/gavinsan33/oc-aibom/releases/latest"><img src="https://img.shields.io/github/v/release/gavinsan33/oc-aibom?label=version" alt="Version"></a>
  <a href="https://github.com/gavinsan33/oc-aibom/blob/main/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/gavinsan33/oc-aibom" alt="Go version"></a>
</p>

A `kubectl`/`oc` plugin for browsing, filtering and comparing the `AIBOM`
custom resources that
[aibom-webhook-service](https://github.com/gavinsan33/aibom-webhook-service)
records for each AI workload run, without digging through
`oc get aibomrun -o yaml`. It also verifies each AIBOM's signature. It uses only
the standard Kubernetes API, and works the same with `kubectl` as with `oc`.

**Full documentation lives in [`docs/`](docs/index.md)** and on the
[docs site](https://gavinsan33.github.io/project-aibom/oc-aibom/). This README
only covers what the plugin does and how to install it.

## Install

With [krew](https://krew.sigs.k8s.io/), using this repo as its own plugin index:

```sh
oc krew index add oc-aibom https://github.com/gavinsan33/oc-aibom.git
oc krew install oc-aibom/aibomrun
```

Or from source (Go 1.22+), which also installs shell completion:

```sh
make install
```

Upgrading from the old `oc aibom` name? Remove that plugin first; see
[Upgrading](docs/install.md#upgrading-from-oc-aibom).

## Usage

```sh
oc aibomrun                               # interactive browser over the list
oc aibomrun list -A --model=granite-3.0-8b
oc aibomrun describe <name>               # readable summary, with signature check
oc aibomrun compare <name> <name> [...]   # side-by-side fields and performance
oc aibomrun diff <name-a> <name-b>        # only what differs
oc aibomrun graph <name> [<name>...]      # stored telemetry, overlaid per run
```

It accepts the usual connection flags (`-n`, `-A`, `--context`, `--kubeconfig`, ...).

![Two runs compared across GPU, CPU, memory and network panels](docs/img/graph-grid-overview.png)

## Documentation

- [Install](docs/install.md): krew, source builds, shell completion, upgrading
- [list](docs/usage/list.md): filters, sorting, and the interactive browser's keys
- [describe](docs/usage/describe.md): every section, and what each signature and telemetry status means
- [diff and compare](docs/usage/diff-compare.md)
- [graph](docs/usage/graph.md): the chart view, its keys, and text output
- [Flags](docs/reference/flags.md)
- [Development](docs/reference/development.md): Makefile targets, project layout, releasing
