.PHONY: build test test-v clean install deps fmt lint coverage help

# Binary name
BINARY := devbox

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOFMT := $(GOCMD) fmt
GOMOD := $(GOCMD) mod
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -s -w

# Default target
all: build

## build: Build the binary
build:
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY) .

## build-debug: Build with debug symbols
build-debug:
	$(GOBUILD) -o $(BINARY) .

## test: Run tests
test:
	$(GOTEST) ./...

## test-v: Run tests with verbose output
test-v:
	$(GOTEST) -v ./...

## test-race: Run tests with race detector
test-race:
	$(GOTEST) -race ./...

## coverage: Run tests with coverage report
coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## fmt: Format code
fmt:
	$(GOFMT) ./...

## vet: Run go vet
vet:
	$(GOVET) ./...

## lint: Run golangci-lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

## install: Install to GOPATH/bin
install:
	$(GOCMD) install .

## uninstall: Remove from GOPATH/bin
uninstall:
	rm -f $(shell go env GOPATH)/bin/$(BINARY)

## run: Build and run with arguments (usage: make run ARGS="init")
run: build
	./$(BINARY) $(ARGS)

## docker-test: Test Docker connectivity
docker-test:
	@docker info > /dev/null 2>&1 && echo "Docker is running" || echo "Docker is not running"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'
