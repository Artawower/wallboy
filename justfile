# Wallboy - CLI wallpaper manager
# https://github.com/casey/just

set shell := ["bash", "-cu"]

# Default recipe - show available commands
default:
    @just --list

# Build the binary
build:
    go build -o wallboy ./cmd/wallboy

# Build with version info
build-release version="dev":
    go build -ldflags "-s -w -X main.Version={{version}}" -o wallboy ./cmd/wallboy

# Run tests
test:
    go test -v ./...

# Run tests with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Format code
fmt:
    go fmt ./...
    gofumpt -l -w .

# Lint code
lint:
    golangci-lint run

# Tidy dependencies
tidy:
    go mod tidy

# Clean build artifacts
clean:
    rm -f wallboy
    rm -f coverage.out coverage.html
    rm -rf dist/

# Install to GOPATH/bin
install:
    go install ./cmd/wallboy

# Install locally to /usr/local/bin
install-local: build
    sudo cp wallboy /usr/local/bin/

# Uninstall from /usr/local/bin
uninstall-local:
    sudo rm -f /usr/local/bin/wallboy

# Run wallboy (pass arguments after --)
run *args:
    go run ./cmd/wallboy {{args}}

# Initialize wallboy config
init:
    go run ./cmd/wallboy init

# Set next wallpaper
next:
    go run ./cmd/wallboy next

# Show current wallpaper info
info:
    go run ./cmd/wallboy info

# List datasources
sources:
    go run ./cmd/wallboy sources

# Analyze colors of current wallpaper
colors:
    go run ./cmd/wallboy colors

# Build for all platforms
build-all:
    GOOS=darwin GOARCH=amd64 go build -o dist/wallboy-darwin-amd64 ./cmd/wallboy
    GOOS=darwin GOARCH=arm64 go build -o dist/wallboy-darwin-arm64 ./cmd/wallboy
    GOOS=linux GOARCH=amd64 go build -o dist/wallboy-linux-amd64 ./cmd/wallboy
    GOOS=linux GOARCH=arm64 go build -o dist/wallboy-linux-arm64 ./cmd/wallboy
    @echo "Binaries built in dist/"

# Create release archives
release version: build-all
    mkdir -p dist/release
    cd dist && tar -czf release/wallboy-{{version}}-darwin-amd64.tar.gz wallboy-darwin-amd64
    cd dist && tar -czf release/wallboy-{{version}}-darwin-arm64.tar.gz wallboy-darwin-arm64
    cd dist && tar -czf release/wallboy-{{version}}-linux-amd64.tar.gz wallboy-linux-amd64
    cd dist && tar -czf release/wallboy-{{version}}-linux-arm64.tar.gz wallboy-linux-arm64
    @echo "Release archives created in dist/release/"

# Show config file location
config-path:
    @echo "Config: ~/.config/wallboy/config.toml"
    @cat ~/.config/wallboy/config.toml 2>/dev/null || echo "(not found - run 'just init')"

# Edit config file
config-edit:
    ${EDITOR:-vim} ~/.config/wallboy/config.toml

# Show state file
state:
    @cat ~/.config/wallboy/state.json 2>/dev/null | jq . || echo "(not found)"

# Clear temp files
clear-temp:
    rm -rf $(go run ./cmd/wallboy 2>/dev/null || echo "/tmp/wallboy")/*
    @echo "Temp files cleared"

# Development: watch and rebuild on changes (requires watchexec)
watch:
    watchexec -e go -r -- go build -o wallboy ./cmd/wallboy

# Check if all tools are installed
check-tools:
    @command -v go >/dev/null 2>&1 || { echo "go: not found"; exit 1; }
    @command -v golangci-lint >/dev/null 2>&1 || echo "golangci-lint: not found (optional, for linting)"
    @command -v gofumpt >/dev/null 2>&1 || echo "gofumpt: not found (optional, for formatting)"
    @command -v watchexec >/dev/null 2>&1 || echo "watchexec: not found (optional, for watch mode)"
    @command -v jq >/dev/null 2>&1 || echo "jq: not found (optional, for state viewing)"
    @echo "Required tools check complete"

# Install development tools
install-tools:
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install mvdan.cc/gofumpt@latest
    @echo "Dev tools installed"
