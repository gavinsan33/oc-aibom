# oc-aibom

A `kubectl`/`oc` plugin that makes it easier to filter through and compare [AIBOM](https://github.com/gavinsan33/aibom-webhook-service) (`aibom.io/v1alpha1`) custom resources than raw `oc get aibom -o yaml`, using only the standard Kubernetes API.

Examples use `oc`, but `oc` is a superset of `kubectl` and both use the same plugin mechanism, so every command works identically with `kubectl`.

## Commands

| Command | What it does |
|---------|--------------|
| `oc aibom list` | List AIBOMs with filters and performance sorting |
| `oc aibom describe <name>` | Human-readable summary of one AIBOM, including signature status |
| `oc aibom diff <name-a> <name-b>` | Field-by-field comparison of two AIBOMs |
| `oc aibom compare <name> <name> [<name>...]` | Side-by-side performance table across runs |
| `oc aibom graph <name> [<name>...]` | Interactive terminal charts of the stored telemetry time series, with runs overlaid |

Start with [Install](install.md).
