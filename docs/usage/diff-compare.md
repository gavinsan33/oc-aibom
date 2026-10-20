# oc aibom compare and diff

## compare

```sh
oc aibom compare <name> <name> [<name>...]
oc aibom compare <name> <name> --changed
```

A side-by-side comparison of two or more AIBOMs, matching the console plugin's
compare view. It prints two tables:

- **Fields** — one row per compared field (job, model config, dataset
  declaration and drift, git provenance, hardware/driver environment, AIBOM
  version), one column per run. Fields whose values aren't identical across
  every run are marked with `*`.
- **Hardware Performance** — one row per metric (GPU utilization, memory and
  power, CPU and memory usage, network RX/TX, storage read/write), one column
  per run. When exactly two AIBOMs are compared, `DELTA` and `CHANGE` columns
  quantify the second relative to the first (sign-colored).

`--changed` hides the field rows that are identical across runs, leaving only
what differs.

## diff

```sh
oc aibom diff <name-a> <name-b>
```

A convenience alias for `compare <name-a> <name-b> --changed`: the same two
tables, but the Fields table shows only the fields that differ.
