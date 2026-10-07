# oc aibom describe

```sh
oc aibom describe <name>
```

Prints a human-readable summary of a single AIBOM instead of a raw YAML dump:

- model
- dataset, declared versus auto-detected, flagging mismatches
- source provenance
- environment
- resource utilization (GPU, CPU, memory and network averages, plus any Grafana links)

## Performance detail

If the AIBOM was compiled with per-metric detail, a "Performance Detail" section shows each metric's min, avg, max and p95, and its first, middle and last-third breakdown across the run. A trend arrow (↑, ↓, →) flags runs that ramped up, throttled down, or held steady, which a single run-wide average can't show.

## Inference performance

For an inference AIBOM whose serving engine has its own telemetry (vLLM today), an "Inference Performance" section shows the serving-level metrics: time to first token, inter-token latency, queue depth, KV-cache usage and throughput. It uses the same min, avg, max, p95 and segment detail.

This section is separate from resource utilization because it describes the serving application's behavior, not the hardware underneath. It's absent on an AIBOM that predates the field, or whose serving engine isn't covered yet.

## Pre-pulled models

For a KServe model pre-pulled onto a PVC, the Model section also shows how the name was determined (`via: model_files_readme` means it came from the model's own README rather than the PVC folder name), plus the pinned `Revision`, `Base Model` and `Size` read from the model's files.

This is identification, not verification: nothing checks the files against the weights. `diff` flags a changed `model.revision`.

## Signature

A `Signature:` line reports whether the AIBOM's Ed25519 signature checks out:

| Status | Meaning |
|--------|---------|
| `✓ Verified` (green) | The signature matches the AIBOM's data, and the embedded public key matches the one the cluster currently publishes. The only status to read as "trust this". |
| `not signed` | No signature present, for example an AIBOM created before signing shipped, or in a namespace with no signing key. Not itself a bad sign. |
| `signed (unconfirmed)` (yellow) | The signature is internally consistent, but the cluster's published key couldn't be checked (no RBAC, no network, or the ConfigMap doesn't exist yet). It proves the signature matches some key, not that the key is the cluster's real one. |
| `⚠ KEY MISMATCH` (yellow) | The signature is valid, but the embedded key doesn't match the cluster's current published key. This can follow a legitimate key rotation, or mean the embedded key was forged. |
| `✗ INVALID SIGNATURE` (red) | The signature does not match the data: tampering or corruption. |

## Telemetry series

When the AIBOM has a persisted telemetry time series, a `Telemetry Series:` line reports whether that object's `spec.seriesJson` still hashes to the sha256 recorded in the signed data. AIBOMs without a stored series print no such line.

| Status | Meaning |
|--------|---------|
| `✓ Verified` (green) | The content matches the digest, and the AIBOM's own signature is `✓ Verified`, so the digest itself is authenticated. |
| `digest matches (AIBOM not verified)` (yellow) | The content matches, but the AIBOM's signature isn't verified. The digest lives inside the AIBOM's data, so this only shows the series wasn't changed independently of the AIBOM. |
| `✗ DIGEST MISMATCH` (red) | The object's content or size no longer matches the signed digest: it was altered or replaced afterward. |
| `missing` (yellow) | The referenced object no longer exists. |
| `unconfirmed` (yellow) | It couldn't be checked: no RBAC to read `aibomtelemetries`, no cluster, or a reference shape this version doesn't understand. |
