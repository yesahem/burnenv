.PHONY: build install run serve test clean deps help

BINARY_NAME := burnenv

# Default target
help:
	@echo "BurnEnv - Zero-retention, ephemeral secret-sharing"
	@echo ""
	@echo "Usage:"
	@echo "  make build     Build the binary"
	@echo "  make install   Install to $$(go env GOPATH)/bin"
	@echo "  make run       Run the server (alias: make serve)"
	@echo "  make test      Run tests"
	@echo "  make clean     Remove built binary"
	@echo "  make deps      Download dependencies"
	@echo ""

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) .

install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	@mkdir -p $(GOPATH)/bin
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/

run: serve

serve:
	@echo "Starting BurnEnv server..."
	go run . serve

test:
	go test -v ./...

clean:
	rm -f bin/$(BINARY_NAME)

deps:
	go mod download
	go mod tidy
