.PHONY: build test vet lint clean

# Build the CLI binary
build:
	go build -o bin/tfclassify ./cmd/tfclassify

# Run all tests across workspace
test:
	go test ./...

# Run go vet across workspace
vet:
	go vet ./...

# Run linter across workspace
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean ./...
