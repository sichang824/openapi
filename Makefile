.DEFAULT_GOAL := help
.PHONY: help install uninstall build build-cli build-cli-linux build-cli-darwin build-cli-windows build-cli-windows-arm64 test test-windows-build test-windows-installer

SKILL_NAME ?= $(shell basename $(CURDIR))
CLI_NAME ?= oapi
CLI_CMD ?= ./cmd/$(CLI_NAME)
CLI_BIN_DIR ?= $(CURDIR)/bin
CLI_BIN ?= $(CLI_BIN_DIR)/$(CLI_NAME)
LINUX_AMD64_BIN ?= $(CLI_BIN_DIR)/$(CLI_NAME)-linux-amd64
DARWIN_AMD64_BIN ?= $(CLI_BIN_DIR)/$(CLI_NAME)-darwin-amd64
WINDOWS_AMD64_BIN ?= $(CLI_BIN_DIR)/$(CLI_NAME)-windows-amd64.exe
WINDOWS_ARM64_BIN ?= $(CLI_BIN_DIR)/$(CLI_NAME)-windows-arm64.exe
BIN_DIR ?= $(HOME)/.local/bin

help:
	@echo "oapi CLI Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  make build-cli                                  # Build native CLI to ./bin/$(CLI_NAME)"
	@echo "  make build                                      # Build native + linux/darwin/windows binaries"
	@echo "  make install [BIN_DIR=path]                     # Build and install CLI to BIN_DIR"
	@echo "  make uninstall [BIN_DIR=path]                   # Remove installed CLI from BIN_DIR"
	@echo "  make test                                       # Run go test ./..."
	@echo "  make test-windows-build                         # Run scripts/test-windows-build.sh"
	@echo "  make test-windows-installer                     # Validate scripts/install-windows.ps1 logic"
	@echo ""
	@echo "Variables:"
	@echo "  CLI_NAME=$(CLI_NAME)"
	@echo "  CLI_CMD=$(CLI_CMD)"
	@echo "  CLI_BIN_DIR=$(CLI_BIN_DIR)"
	@echo "  BIN_DIR=$(BIN_DIR)"
	@echo ""
	@echo "Examples:"
	@echo "  make build-cli"
	@echo "  make install BIN_DIR=/usr/local/bin"
	@echo "  make install BIN_DIR=$$HOME/.local/bin"
	@echo "  make test"
	@echo "  make build-cli-linux"
	@echo "  make build-cli-windows-arm64"

install: build-cli
	@echo "Installing CLI to $(BIN_DIR)/$(CLI_NAME)..."
	@mkdir -p "$(BIN_DIR)"
	@install -m 0755 "$(CLI_BIN)" "$(BIN_DIR)/$(CLI_NAME)"
	@echo "Done."

uninstall:
	@echo "Removing CLI from $(BIN_DIR)/$(CLI_NAME)..."
	@rm -f "$(BIN_DIR)/$(CLI_NAME)"
	@echo "Done."

build: build-cli build-cli-linux build-cli-darwin build-cli-windows build-cli-windows-arm64
	@echo "Built native and cross-platform binaries in $(CLI_BIN_DIR)"

build-cli:
	@echo "Building CLI $(CLI_NAME) from $(CLI_CMD)..."
	@mkdir -p "$(CLI_BIN_DIR)"
	@go build -o "$(CLI_BIN)" "$(CLI_CMD)"
	@echo "CLI binary: $(CLI_BIN)"

build-cli-linux:
	@echo "Cross-compiling $(CLI_NAME) for linux/amd64..."
	@mkdir -p "$(CLI_BIN_DIR)"
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$(LINUX_AMD64_BIN)" "$(CLI_CMD)"
	@echo "Linux amd64 binary: $(LINUX_AMD64_BIN)"

build-cli-darwin:
	@echo "Cross-compiling $(CLI_NAME) for darwin/amd64..."
	@mkdir -p "$(CLI_BIN_DIR)"
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o "$(DARWIN_AMD64_BIN)" "$(CLI_CMD)"
	@echo "Darwin amd64 binary: $(DARWIN_AMD64_BIN)"

# CGO_ENABLED=0 keeps cross-builds static and avoids needing a Windows C toolchain.
build-cli-windows:
	@echo "Cross-compiling $(CLI_NAME) for windows/amd64..."
	@mkdir -p "$(CLI_BIN_DIR)"
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$(WINDOWS_AMD64_BIN)" "$(CLI_CMD)"
	@echo "Windows amd64 binary: $(WINDOWS_AMD64_BIN)"

build-cli-windows-arm64:
	@echo "Cross-compiling $(CLI_NAME) for windows/arm64..."
	@mkdir -p "$(CLI_BIN_DIR)"
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o "$(WINDOWS_ARM64_BIN)" "$(CLI_CMD)"
	@echo "Windows arm64 binary: $(WINDOWS_ARM64_BIN)"

test:
	@go test ./...

test-windows-build:
	@./scripts/test-windows-build.sh

test-windows-installer:
	@./scripts/test-windows-installer.sh
