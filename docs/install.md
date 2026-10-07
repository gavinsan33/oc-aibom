# Install

## With krew (preferred)

Install via [krew](https://krew.sigs.k8s.io/), kubectl's plugin manager, using this repository as a self-hosted plugin index:

```sh
oc krew index add oc-aibom https://github.com/gavinsan33/oc-aibom.git
oc krew install oc-aibom/aibom
```

## From source

Requires Go 1.22+:

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

## Shell completion

`kubectl` and `oc` don't call a plugin's own `__complete` command directly. They look on `PATH` for a companion executable named `kubectl_complete-aibom` (or `oc_complete-aibom`), and silently fall back to filename completion if it's missing.

`make install` installs both companion scripts alongside the binary, so `oc aibom <TAB>` and `oc aibom list --<TAB>` work for source installs, as long as your shell has `oc`'s (or `kubectl`'s) own completion sourced:

```sh
source <(oc completion zsh)   # or: source <(oc completion bash)
```

!!! note "krew installs"
    krew only symlinks the manifest's `bin:` entry onto `PATH`, not the completion scripts bundled in the release tarball. To enable completion for a krew install, copy the extracted `kubectl_complete-aibom` and `oc_complete-aibom` files from `~/.krew/store/aibom/<version>/` into `~/.krew/bin`.
