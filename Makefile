BINARY_NAME := ak9s
GO := go
GOLANGCI_LINT := golangci-lint

.PHONY: build clean fmt lint test tidy run help

## build: Build the binary
build:
	$(GO) build -o $(BINARY_NAME) ./main.go

## run: Build and run the binary
run: build
	./$(BINARY_NAME)

## fmt: Format Go source files
fmt:
	$(GO) fmt ./...

## lint: Run golangci-lint
lint:
	$(GOLANGCI_LINT) run ./...

## test: Run tests
test:
	$(GO) test -v ./...

## tidy: Tidy and verify go.mod
tidy:
	$(GO) mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy" && exit 1)

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
