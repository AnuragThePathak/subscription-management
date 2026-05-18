.PHONY: mocks test integration test-all coverage lint vet build clean check

## Generate all mocks via .mockery.yaml
mocks:
	mockery

## Fast unit tests with coverage summary (no race detector)
test:
	go test -coverprofile=coverage.out ./...
	@grep -v "/mocks/" coverage.out > coverage_filtered.out
	@go tool cover -func=coverage_filtered.out | tail -1

## Integration tests only (requires Docker for testcontainers)
integration:
	go test -tags integration -race ./...

## All tests (unit + integration) with race detector and coverage summary
test-all:
	go test -tags integration -race -coverprofile=coverage.out ./...
	@grep -v "/mocks/" coverage.out > coverage_filtered.out
	@go tool cover -func=coverage_filtered.out | tail -1

## Open coverage HTML in the browser (run test or test-all first)
coverage:
	go tool cover -html=coverage_filtered.out

## Run go vet
vet:
	go vet ./...

## Run linter
lint:
	golangci-lint run ./...

## Pre-commit: vet, lint, then all tests with race detector
check: vet lint test-all

## Build the binary
build:
	go build -o bin/app .

## Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage_filtered.out