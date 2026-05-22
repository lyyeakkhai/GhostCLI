# GhostCLI Makefile
# Provides cross-platform build targets and development helpers.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(DATE)

BINARY := ghost
CMD := ./cmd/ghost

PLATFORMS := \
	windows/amd64 \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

.PHONY: all build test clean release $(PLATFORMS)

all: build

# Build for the current platform
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# Run tests
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run tests without race detector (faster)
test-fast:
	go test ./...

# Run linting
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Clean build artifacts
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/
	rm -f coverage.out

# Build for every platform and place binaries in dist/
release: clean
	mkdir -p dist
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		output="dist/$(BINARY)-$$GOOS-$$GOARCH$$ext"; \
		echo "Building $$output ..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $$output $(CMD); \
		shasum -a 256 $$output > $$output.sha256; \
	done

# Individual platform targets
windows/amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(CMD)

linux/amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 $(CMD)

linux/arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 $(CMD)

darwin/amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 $(CMD)

darwin/arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 $(CMD)

# Development helpers
dev:
	go run $(CMD) --verbose

# Install locally (requires Go toolchain)
install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

# Verify module dependencies
verify:
	go mod verify
	go mod tidy

# Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.DEFAULT_GOAL := build
