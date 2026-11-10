# Development

Building from source needs Go 1.22 or later.

## Makefile targets

| Target | What it does |
|--------|--------------|
| `make build` | Build `./kubectl-aibomrun` |
| `make install` | Build, then install the binary and the shell-completion companion scripts to `/usr/local/bin` (override with `INSTALL_DIR=...`) |
| `make uninstall` | Remove the installed binary and completion scripts from `INSTALL_DIR` |
| `make test` | Run the unit tests |
| `make vet` | Run `go vet` |
| `make check` | `vet` and `test` |
| `make fmt` | `gofmt` the repo |
| `make tidy` | `go mod tidy` |
| `make dist` | Cross-compile release tarballs (binary and completion scripts) and `checksums.txt` into `dist/` |
| `make clean` | Remove the built binary and `dist/` |

## Project structure

```
cmd/kubectl-aibomrun/   CLI entrypoint: cobra commands, table and summary output, the interactive browser and graph views
internal/aibom/         AIBOM types, dynamic-client queries, filter, sort and diff logic
completion/             kubectl_complete-aibomrun and oc_complete-aibomrun companion scripts
plugins/aibomrun.yaml   krew manifest; this repo is its own krew index
```

## Releasing

Push a `v*` tag. The `release` workflow builds the archives with `make dist`, publishes them and `checksums.txt` as a GitHub release, then commits the new version and sha256 values to `plugins/aibomrun.yaml` on `main`. Nothing needs editing by hand; `krew install oc-aibom/aibomrun` picks up the release once that commit lands.

To try the krew manifest locally without a release:

```sh
make dist VERSION=v0.0.0-dev
kubectl krew install --manifest=plugins/aibomrun.yaml --archive=dist/kubectl-aibomrun_v0.0.0-dev_linux_amd64.tar.gz
```
