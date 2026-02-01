.PHONY: build run test clean swagger deps

# Variables
BINARY_NAME=claude-monitor
VERSION=2.1.0

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the application
build: swagger
	$(GOBUILD) -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME) .

# Run the application
run: swagger
	$(GORUN) .

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf docs/

# Generate Swagger documentation
swagger:
	@which swag > /dev/null || (echo "Installing swag..." && go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g main.go -o docs

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install development tools
tools:
	go install github.com/swaggo/swag/cmd/swag@latest

# Format code
fmt:
	$(GOCMD) fmt ./...

# Vet code
vet:
	$(GOCMD) vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Build for multiple platforms
build-all: swagger
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-windows-amd64.exe .

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application (generates swagger first)"
	@echo "  run        - Run the application"
	@echo "  test       - Run tests"
	@echo "  clean      - Remove build artifacts"
	@echo "  swagger    - Generate Swagger documentation"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  tools      - Install development tools"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  lint       - Run linter"
	@echo "  build-all  - Build for multiple platforms"
