.PHONY: test test-v test-race test-cover test-run cover-html cover-func cover-check build clean

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage and print summary
test-cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

# Generate HTML coverage report and open it
cover-html: test-cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Print per-function coverage breakdown
cover-func: test-cover
	go tool cover -func=coverage.out

# Run a specific test by name: make test-run T=TestWalletCreate
test-run:
	go test -v -run $(T) ./...

# Enforce minimum coverage threshold (default 80%)
COVER_MIN ?= 80
cover-check: test-cover
	@coverage=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	if awk "BEGIN {exit !($$coverage < $(COVER_MIN))}"; then \
		echo "FAIL: coverage $$coverage% < $(COVER_MIN)% threshold"; \
		exit 1; \
	else \
		echo "OK: coverage $$coverage% >= $(COVER_MIN)% threshold"; \
	fi

# Build the binary
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/peth ./cmd/peth

# Clean build artifacts and coverage files
clean:
	rm -rf bin/ coverage.out coverage.html
