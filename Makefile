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

check: lint test
	@echo "==> All checks passed!"
