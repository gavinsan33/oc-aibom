BINARY       := kubectl-aibom
CMD          := ./cmd/kubectl-aibom
INSTALL_DIR  := /usr/local/bin
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST_DIR     := dist
PLATFORMS    := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
COMPLETIONS  := completion/kubectl_complete-aibom completion/oc_complete-aibom

.PHONY: all
all: build

.PHONY: build
build:
	go build -o $(BINARY) $(CMD)

.PHONY: install
install: build
	sudo install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	sudo install -m 0755 completion/kubectl_complete-aibom $(INSTALL_DIR)/kubectl_complete-aibom
	sudo install -m 0755 completion/kubectl_complete-aibom $(INSTALL_DIR)/oc_complete-aibom

.PHONY: uninstall
uninstall:
	sudo rm -f $(INSTALL_DIR)/$(BINARY) $(INSTALL_DIR)/kubectl_complete-aibom $(INSTALL_DIR)/oc_complete-aibom

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
# in the layout krew expects: kubectl-aibom_<version>_<os>_<arch>.tar.gz
# containing the kubectl-aibom (or kubectl-aibom.exe) binary plus the
# kubectl_complete-aibom/oc_complete-aibom companion completion scripts.
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
		cp completion/kubectl_complete-aibom $(DIST_DIR)/kubectl_complete-aibom$(EXT) && \
		cp completion/kubectl_complete-aibom $(DIST_DIR)/oc_complete-aibom$(EXT) && \
		tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz $(BINARY)$(EXT) kubectl_complete-aibom$(EXT) oc_complete-aibom$(EXT) && \
		rm -f $(DIST_DIR)/$(BINARY)$(EXT) $(DIST_DIR)/kubectl_complete-aibom$(EXT) $(DIST_DIR)/oc_complete-aibom$(EXT) && \
	) true
	cd $(DIST_DIR) && sha256sum *.tar.gz > checksums.txt
	@echo "Built release artifacts for $(VERSION) in $(DIST_DIR)/"
