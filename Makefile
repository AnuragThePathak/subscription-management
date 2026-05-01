.PHONY: mocks test integration lint build

## Generate all mocks via .mockery.yaml
mocks:
	mockery

## Run unit tests
test:
	go test ./...

## Run integration tests (requires external services)
integration:
	go test -tags integration ./...

## Run linter
lint:
	golangci-lint run ./...

## Build the binary
build:
	go build -o bin/app ./...
