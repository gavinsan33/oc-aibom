# Flags

`oc-aibom` is built with `k8s.io/cli-runtime`'s `genericclioptions`, so it accepts the same connection flags `oc` and `kubectl` do: `--kubeconfig`, `--context`, `--namespace`/`-n`, `--server`, `--token`, and so on. No plugin-specific configuration is needed.

For the filter and sort flags, see [list](../usage/list.md).

## Implementation notes

There is no generated Go clientset for the `AIBOM` custom resource, since it is created directly from Python in `aibom-webhook-service`. The plugin therefore talks to the API through `k8s.io/client-go`'s dynamic client against `aibom.io/v1alpha1`, resource `aiboms`, and decodes `spec.data` into Go structs for typed filtering and diffing.
