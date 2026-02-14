.PHONY: build build-all test vet lint clean generate-roles

# Build the CLI binary
build:
	go build -o bin/tfclassify ./cmd/tfclassify

# Build all binaries
build-all: build
	go build -o bin/tfclassify-plugin-azurerm ./plugins/azurerm

# Run all tests across workspace
test:
	go test ./...

# Run go vet across workspace
vet:
	go vet ./...

# Run linter across workspace
lint:
	golangci-lint run ./...

# Regenerate Azure built-in role data from AzAdvertizer CSV
generate-roles:
	curl -sL --compressed https://www.azadvertizer.net/azrolesadvertizer-comma.csv | \
		go run tools/csv2roles/main.go > plugins/azurerm/roledata/roles.json

# Clean build artifacts
clean:
	rm -rf bin/
	go clean ./...
