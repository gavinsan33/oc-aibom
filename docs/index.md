# oc-aibom

A `kubectl`/`oc` plugin that makes it easier to filter through and compare [AIBOM](https://github.com/gavinsan33/aibom-webhook-service) (`aibom.io/v1alpha1`) custom resources than raw `oc get aibomrun -o yaml`, using only the standard Kubernetes API.

Examples use `oc`, but `oc` is a superset of `kubectl` and both use the same plugin mechanism, so every command works identically with `kubectl`.

## Commands

| Command | What it does |
|---------|--------------|
| `oc aibomrun` | Interactive full-screen browser over the list — the default on a terminal; drill into describe, compare and graph |
| `oc aibomrun list` | List AIBOMs with filters and performance sorting |
| `oc aibomrun describe <name>` | Human-readable summary of one AIBOM, including signature status |
| `oc aibomrun diff <name-a> <name-b>` | Field-by-field comparison of two AIBOMs |
| `oc aibomrun compare <name> <name> [<name>...]` | Side-by-side performance table across runs |
| `oc aibomrun graph <name> [<name>...]` | Interactive terminal charts of the stored telemetry time series, with runs overlaid |

This plugin reads `aiboms.aibom.io`, the run records written by aibom-webhook-service. [k8s-aibom](https://github.com/GoogleCloudPlatform/k8s-aibom) defines a different `AIBOM` kind (`aibom.k8saibom.dev`) and has its own `kubectl aibom` plugin; the two can be installed side by side.

Start with [Install](install.md).
