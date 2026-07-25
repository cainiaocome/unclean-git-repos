## Makefile for unclean-git-repos

# Binary name and output location.
BINARY      := unclean-git-repos
BIN_DIR     := bin
BIN         := $(BIN_DIR)/$(BINARY)

# Release artifacts directory and the platforms we cross-compile for.
DIST_DIR    := dist
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Install location (override with: make install PREFIX=/usr/local).
PREFIX      ?= $(HOME)/.local
INSTALL_DIR := $(PREFIX)/bin

# Go tooling.
GO          ?= go
GOFLAGS     ?=

# Embed version info from git into the binary (falls back to "dev").
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)

# Every recipe below is a task, not a file.
.PHONY: all build run install uninstall test vet fmt tidy clean help \
        release-build release bump-patch bump-minor bump-major version

## all: format, vet and build (default target)
all: fmt vet build

## build: compile the binary into ./bin
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN) .
	@echo "built $(BIN) ($(VERSION))"

## run: build and run against the current directory (pass args with ARGS=...)
run: build
	./$(BIN) $(ARGS)

## install: build and install the binary to $(INSTALL_DIR)
install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_DIR)/$(BINARY)
	@echo "installed to $(INSTALL_DIR)/$(BINARY)"

## uninstall: remove the installed binary
uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "removed $(INSTALL_DIR)/$(BINARY)"

## test: run the test suite
test:
	$(GO) test $(GOFLAGS) ./...

## vet: run go vet static checks
vet:
	$(GO) vet ./...

## fmt: format all Go source files
fmt:
	$(GO) fmt ./...

## tidy: sync go.mod / go.sum
tidy:
	$(GO) mod tidy

## version: print the version that would be embedded in a build
version:
	@echo $(VERSION)

## release-build: cross-compile release binaries for all platforms into ./dist
release-build:
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
	  os=$${platform%/*}; arch=$${platform#*/}; \
	  ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	  out=$(DIST_DIR)/$(BINARY)-$(VERSION)-$$os-$$arch$$ext; \
	  echo "building $$out"; \
	  GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
	    $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $$out . || exit 1; \
	done
	@echo "release binaries in $(DIST_DIR)/"

## release: tag the current commit and push it to trigger the release workflow (make release V=vX.Y.Z)
release:
	@test -n "$(V)" || { echo "usage: make release V=vX.Y.Z"; exit 1; }
	@echo "$(V)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "V must look like vX.Y.Z"; exit 1; }
	git tag -a "$(V)" -m "Release $(V)"
	git push origin "$(V)"
	@echo "pushed tag $(V) — the Release workflow will build and publish it"

## bump-patch: tag & push the next patch version (vX.Y.Z -> vX.Y.Z+1)
## bump-minor: tag & push the next minor version (vX.Y.Z -> vX.Y+1.0)
## bump-major: tag & push the next major version (vX.Y.Z -> vX+1.0.0)
bump-patch bump-minor bump-major:
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0); \
	v=$${latest#v}; \
	major=$$(echo $$v | cut -d. -f1); minor=$$(echo $$v | cut -d. -f2); patch=$$(echo $$v | cut -d. -f3); \
	case $@ in \
	  bump-major) major=$$((major+1)); minor=0; patch=0;; \
	  bump-minor) minor=$$((minor+1)); patch=0;; \
	  bump-patch) patch=$$((patch+1));; \
	esac; \
	new=v$$major.$$minor.$$patch; \
	echo "bumping $$latest -> $$new"; \
	$(MAKE) release V=$$new

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
	$(GO) clean
	@echo "cleaned"

## help: list available targets
help:
	@echo "Available targets:"
	@grep -E '^## [a-z-]+:' $(MAKEFILE_LIST) | sed 's/## /  /'
