.PHONY: all test build clean fmt lint install bench snapshot release test-docs

# Build information
BINARY_NAME=gismo
GO=go
GOFLAGS=-trimpath

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS=-s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE) \
	-X main.builtBy=make

all: fmt lint test test-docs build

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/gismo
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-init ./cmd/gismo-init
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-show ./cmd/gismo-show
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-registry ./cmd/gismo-registry
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-package ./cmd/gismo-package

test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

bench:
	$(GO) test -bench=. -benchmem ./...

fmt:
	$(GO) fmt ./...
	gofmt -s -w .

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found, installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

install:
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/gismo
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/gismo-init
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/gismo-show
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/gismo-registry
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/gismo-package

clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-init
	rm -f $(BINARY_NAME)-show
	rm -f $(BINARY_NAME)-registry
	rm -f $(BINARY_NAME)-package
	rm -f coverage.out
	rm -rf dist/
	$(GO) clean -cache

deps:
	$(GO) mod download
	$(GO) mod tidy

coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html

# Documentation testing
test-docs:
	@echo "Testing documentation examples..."
	$(GO) test -v ./docs/testable/...

# GoReleaser targets
snapshot:
	@command -v goreleaser > /dev/null || (echo "goreleaser not found. Install from https://goreleaser.com" && exit 1)
	goreleaser release --snapshot --clean

release:
	@command -v goreleaser > /dev/null || (echo "goreleaser not found. Install from https://goreleaser.com" && exit 1)
	goreleaser release --clean