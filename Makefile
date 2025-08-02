.PHONY: all build test test-race clean fmt lint mod-tidy help

# Default target
all: clean build test

# Build the coreth plugin
build:
	@echo "Building coreth..."
	@./scripts/build.sh

# Run tests without race detection
test:
	@echo "Running tests..."
	@NO_RACE=1 ./scripts/build_test.sh

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@./scripts/build_test.sh

# Run e2e tests
test-e2e:
	@echo "Running e2e tests..."
	@./scripts/tests.e2e.sh

# Run ginkgo warp tests
test-warp:
	@echo "Running warp tests..."
	@./scripts/run_ginkgo_warp.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out test.out

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linters
lint:
	@echo "Running linters..."
	@./scripts/lint.sh

# Update dependencies
mod-tidy:
	@echo "Updating dependencies..."
	@go mod tidy

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	@./scripts/coverage.sh

# Build with Avalanchego
build-with-avalanchego:
	@echo "Building with Avalanchego..."
	@./scripts/build_avalanchego_with_coreth.sh

# Help
help:
	@echo "Available targets:"
	@echo "  all                   - Clean, build, and test"
	@echo "  build                 - Build the coreth plugin"
	@echo "  test                  - Run tests without race detection"
	@echo "  test-race             - Run tests with race detection"
	@echo "  test-e2e              - Run e2e tests"
	@echo "  test-warp             - Run ginkgo warp tests"
	@echo "  clean                 - Clean build artifacts"
	@echo "  fmt                   - Format code"
	@echo "  lint                  - Run linters"
	@echo "  mod-tidy              - Update dependencies"
	@echo "  coverage              - Generate coverage report"
	@echo "  build-with-avalanchego - Build with Avalanchego"
	@echo "  help                  - Show this help message"