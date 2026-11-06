BINARY       := kubectl-aibomrun
CMD          := ./cmd/kubectl-aibomrun
INSTALL_DIR  := /usr/local/bin
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST_DIR     := dist
PLATFORMS    := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
COMPLETIONS  := completion/kubectl_complete-aibomrun completion/oc_complete-aibomrun

.PHONY: all
all: build

.PHONY: build
build:
	go build -o $(BINARY) $(CMD)

.PHONY: install
install: build
	sudo install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	sudo install -m 0755 completion/kubectl_complete-aibomrun $(INSTALL_DIR)/kubectl_complete-aibomrun
	sudo install -m 0755 completion/kubectl_complete-aibomrun $(INSTALL_DIR)/oc_complete-aibomrun

.PHONY: uninstall
uninstall:
	sudo rm -f $(INSTALL_DIR)/$(BINARY) $(INSTALL_DIR)/kubectl_complete-aibomrun $(INSTALL_DIR)/oc_complete-aibomrun

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: check
check: vet test

.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)

# dist builds a release tarball per PLATFORMS entry, plus a checksums.txt,
# in the layout krew expects: kubectl-aibomrun_<version>_<os>_<arch>.tar.gz
# containing the kubectl-aibomrun (or kubectl-aibomrun.exe) binary plus the
# kubectl_complete-aibomrun/oc_complete-aibomrun companion completion scripts.
# krew only symlinks the manifest's `bin:` entry onto PATH, so the
# completion scripts are inert for krew installs unless a user also copies
# them onto PATH themselves (e.g. into ~/.krew/bin) — see README.
.PHONY: dist
dist: clean
	mkdir -p $(DIST_DIR)
	$(foreach platform,$(PLATFORMS), \
		$(eval OS := $(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(platform)))) \
		$(eval EXT := $(if $(filter windows,$(OS)),.exe,)) \
		GOOS=$(OS) GOARCH=$(ARCH) go build -o $(DIST_DIR)/$(BINARY)$(EXT) $(CMD) && \
		cp completion/kubectl_complete-aibomrun $(DIST_DIR)/kubectl_complete-aibomrun$(EXT) && \
		cp completion/kubectl_complete-aibomrun $(DIST_DIR)/oc_complete-aibomrun$(EXT) && \
		tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz $(BINARY)$(EXT) kubectl_complete-aibomrun$(EXT) oc_complete-aibomrun$(EXT) && \
		rm -f $(DIST_DIR)/$(BINARY)$(EXT) $(DIST_DIR)/kubectl_complete-aibomrun$(EXT) $(DIST_DIR)/oc_complete-aibomrun$(EXT) && \
	) true
	cd $(DIST_DIR) && sha256sum *.tar.gz > checksums.txt
	@echo "Built release artifacts for $(VERSION) in $(DIST_DIR)/"
