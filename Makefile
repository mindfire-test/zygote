.PHONY: all build test lint clean check

all: check build

build:
	@echo "==> Building zyg CLI..."
	@go build -o bin/zyg cmd/zyg/main.go

test:
	@echo "==> Running tests..."
	@go test -v -race -cover ./...

lint:
	@echo "==> Running golangci-lint..."
	@golangci-lint run

clean:
	@echo "==> Cleaning..."
	@rm -rf bin/
	@go clean

conformance:
	@echo "==> Running format and conformance test suite..."
	@go test -v -race ./spec/conformance/...

check: lint test conformance
	@echo "==> Running TTFV walkthrough..."
	@bash scripts/ttfv_test.sh
	@echo "==> All checks passed!"
