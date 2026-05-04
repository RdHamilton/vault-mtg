# services/daemon/Makefile
#
# Binary naming: mtga-companion-daemon-<os>-<arch>[.exe]
# Must match the artifact names defined in .github/workflows/daemon-release.yml.

.PHONY: build build-all test lint fmt clean

BINARY_PREFIX := mtga-companion-daemon
BIN_DIR       := bin

# Default: build for the local machine.
build:
	go build -v -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_PREFIX) ./cmd/daemon

# Cross-compile all release targets.
build-all: build-windows-amd64 build-darwin-arm64 build-darwin-amd64

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	  go build -v -ldflags="-s -w" \
	  -o $(BIN_DIR)/$(BINARY_PREFIX)-windows-amd64.exe ./cmd/daemon

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
	  go build -v -ldflags="-s -w" \
	  -o $(BIN_DIR)/$(BINARY_PREFIX)-darwin-arm64 ./cmd/daemon

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
	  go build -v -ldflags="-s -w" \
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
