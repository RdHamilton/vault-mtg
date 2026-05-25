# services/daemon/Makefile
#
# Binary naming: vaultmtg-daemon-<os>-<arch>[.exe]
# Must match the artifact names defined in .github/workflows/daemon-release.yml.
# ADR-022 Phase 6 (#1764) cutover 2026-05-25 removed the legacy
# mtga-companion-daemon-* dual-naming targets.

.PHONY: build build-all test lint fmt clean

BINARY_PREFIX := vaultmtg-daemon
BIN_DIR       := bin

# VERSION: strip the "daemon/" prefix from tags like "daemon/v0.3.0" -> "0.3.0".
# Falls back to "dev" if no matching tag is found.
RAW_VERSION   := $(shell git describe --tags --match 'daemon/v*' --abbrev=0 2>/dev/null)
VERSION       := $(if $(RAW_VERSION),$(patsubst daemon/v%,%,$(RAW_VERSION)),dev)

LDFLAGS := -s -w -X main.Version=$(VERSION)

# Default: build for the local machine.
build:
	go build -v -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_PREFIX) ./cmd/daemon

# Cross-compile all release targets.
build-all: build-windows-amd64 build-darwin-arm64 build-darwin-amd64

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	  go build -v -ldflags="$(LDFLAGS)" \
	  -o $(BIN_DIR)/$(BINARY_PREFIX)-windows-amd64.exe ./cmd/daemon

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
	  go build -v -ldflags="$(LDFLAGS)" \
	  -o $(BIN_DIR)/$(BINARY_PREFIX)-darwin-arm64 ./cmd/daemon

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
	  go build -v -ldflags="$(LDFLAGS)" \
	  -o $(BIN_DIR)/$(BINARY_PREFIX)-darwin-amd64 ./cmd/daemon

test:
	go test -race ./...

lint:
	gofumpt -l .
	go vet ./...

fmt:
	gofumpt -w ./...

clean:
	rm -rf $(BIN_DIR)
