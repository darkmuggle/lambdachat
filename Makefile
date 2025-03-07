# LambdaChat Slackbot Makefile

.PHONY: all build clean test run-slackbot run-direct

# Default target
all: build

# Build all binaries
build: build-slackbot build-cli

# Build the slackbot binary
build-slackbot:
	@echo "Building slackbot..."
	go build -o bin/slackbot ./cmd/slackbot

# Build the CLI binary
build-cli:
	@echo "Building CLI..."
	go build -o bin/cli ./cmd/cli

# Clean up binaries
clean:
	@echo "Cleaning up..."
	rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run the slackbot example
run-slackbot:
	@echo "Running slackbot example..."
	go run ./examples/slackbot/main.go

# Run the direct example
run-direct:
	@echo "Running direct example..."
	go run ./examples/direct/main.go

# Create directories
init:
	@echo "Creating directories..."
	mkdir -p bin
