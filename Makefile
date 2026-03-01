# Makefile for YouTube Downloader

# Variables
BINARY_NAME=tubexxi-scraper
MAIN_PATH=./cmd/main.go
BUILD_DIR=./build

# Colors
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: help run build clean install test deps

help: ## Tampilkan bantuan
	@echo "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•"
	@echo "  YouTube Downloader - Makefile Commands"
	@echo "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  ${GREEN}%-15s${NC} %s\n", $$1, $$2}'
	@echo "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•"

deps: ## Install dependencies
	@echo "${YELLOW}Installing dependencies...${NC}"
	go mod download
	go mod tidy
	@echo "${GREEN}Dependencies installed!${NC}"

run: ## Run aplikasi
	@echo "${YELLOW}Running application...${NC}"
	cd cmd && go run main.go

build: ## Build binary
	@echo "${YELLOW}Building binary...${NC}"
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "${GREEN}Build complete! Binary: $(BUILD_DIR)/$(BINARY_NAME)${NC}"

build-linux: ## Build untuk Linux
	@echo "${YELLOW}Building for Linux...${NC}"
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux $(MAIN_PATH)
	@echo "${GREEN}Linux build complete!${NC}"

build-windows: ## Build untuk Windows
	@echo "${YELLOW}Building for Windows...${NC}"
	mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME).exe $(MAIN_PATH)
	@echo "${GREEN}Windows build complete!${NC}"

build-mac: ## Build untuk macOS
	@echo "${YELLOW}Building for macOS...${NC}"
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-mac $(MAIN_PATH)
	@echo "${GREEN}macOS build complete!${NC}"

build-all: build-linux build-windows build-mac ## Build untuk semua platform
	@echo "${GREEN}All builds complete!${NC}"

install: build ## Install binary ke $GOPATH/bin
	@echo "${YELLOW}Installing binary...${NC}"
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "${GREEN}Installed to $(GOPATH)/bin/$(BINARY_NAME)${NC}"

clean: ## Hapus build artifacts
	@echo "${YELLOW}Cleaning...${NC}"
	rm -rf $(BUILD_DIR)
	rm -rf ./downloads/*
	@echo "${GREEN}Clean complete!${NC}"

test: ## Run tests
	@echo "${YELLOW}Running tests...${NC}"
	go test -v ./...

fmt: ## Format kode
	@echo "${YELLOW}Formatting code...${NC}"
	go fmt ./...
	@echo "${GREEN}Format complete!${NC}"

lint: ## Run linter
	@echo "${YELLOW}Running linter...${NC}"
	golangci-lint run
	@echo "${GREEN}Lint complete!${NC}"

.DEFAULT_GOAL := help