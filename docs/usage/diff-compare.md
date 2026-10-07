# oc aibom diff and compare

## diff

```sh
oc aibom diff <name-a> <name-b>
```

A field-by-field comparison of two AIBOMs covering:

- model config
- dataset declaration and drift
- git provenance
- hardware and driver environment
- a quantified performance table (value, delta and percent change) across GPU utilization, memory and power, CPU and memory usage, and network throughput

When segmented telemetry stats are available, each run's own value is annotated with its within-run trend arrow. That is separate from the delta and change columns, which only compare the two runs' averages against each other.

## compare

```sh
oc aibom compare <name> <name> [<name>...]
```

A side-by-side performance table across two or more AIBOMs, with one column per run. Use it to spot trends across a run history instead of a single pairwise diff.
