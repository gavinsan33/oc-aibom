# Install

## With krew (preferred)

Install via [krew](https://krew.sigs.k8s.io/), kubectl's plugin manager, using this repository as a self-hosted plugin index:

```sh
oc krew index add oc-aibom https://github.com/gavinsan33/oc-aibom.git
oc krew install oc-aibom/aibomrun
```

### Upgrading from `oc aibom`

The plugin used to install as `kubectl-aibom` (krew plugin `oc-aibom/aibom`), invoked as `oc aibom`. That name belongs to [k8s-aibom](https://github.com/GoogleCloudPlatform/k8s-aibom)'s krew plugin, which a `kubectl-aibom` binary on `PATH` would shadow, so it is now `kubectl-aibomrun`, invoked as `oc aibomrun`. Remove the old one and install the new one:

```sh
oc krew uninstall aibom && oc krew update && oc krew install oc-aibom/aibomrun
# or, for a source install:
sudo rm -f /usr/local/bin/kubectl-aibom /usr/local/bin/kubectl_complete-aibom /usr/local/bin/oc_complete-aibom
```

## From source

Requires Go 1.22+:

```sh
make install   # builds and installs to /usr/local/bin (override with INSTALL_DIR=...)
```

Either way, invoke it as a plugin:

```sh
oc aibomrun list
oc aibomrun describe <name>
oc aibomrun diff <name-a> <name-b>
oc aibomrun compare <name> <name> [<name>...]
```

## Shell completion

`kubectl` and `oc` don't call a plugin's own `__complete` command directly. They look on `PATH` for a companion executable named `kubectl_complete-aibomrun` (or `oc_complete-aibomrun`), and silently fall back to filename completion if it's missing.

`make install` installs both companion scripts alongside the binary, so `oc aibomrun <TAB>` and `oc aibomrun list --<TAB>` work for source installs, as long as your shell has `oc`'s (or `kubectl`'s) own completion sourced:

```sh
source <(oc completion zsh)   # or: source <(oc completion bash)
```

!!! note "krew installs"
    krew only symlinks the manifest's `bin:` entry onto `PATH`, not the completion scripts bundled in the release tarball. To enable completion for a krew install, copy the extracted `kubectl_complete-aibomrun` and `oc_complete-aibomrun` files from `~/.krew/store/aibomrun/<version>/` into `~/.krew/bin`.
