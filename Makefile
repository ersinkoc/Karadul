.PHONY: build build-all test test-race test-cover clean install lint fmt vet help docker-build docker-compose-up release watch setup-homebrew ci check-all web-test web-test-cover web-install web-dev web-build web-lint web-preview build-with-web

# Build variables
BINARY_NAME=karadul
VERSION?=$(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
BUILD_DIR=build
WEB_DIR=web

# Default target
.DEFAULT_GOAL := help

help: ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary for current platform
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/karadul

build-all: ## Build for all platforms (linux, darwin, windows, freebsd, openbsd)
	@mkdir -p $(BUILD_DIR)
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/karadul
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/karadul
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7 ./cmd/karadul
	@echo "Building for macOS..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/karadul
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/karadul
	@echo "Building for Windows..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/karadul
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/karadul
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-x86.exe ./cmd/karadul
	@echo "Building for BSD..."
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd-amd64 ./cmd/karadul
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-openbsd-amd64 ./cmd/karadul
	@echo "✅ All builds complete in $(BUILD_DIR)/"

test: ## Run tests
	go test -v -count=1 ./...

test-race: ## Run tests with race detector
	go test -race -count=1 ./...

test-cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-cover-html: ## Generate HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

lint: ## Run linters (requires golangci-lint)
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

check: fmt vet test-race ## Run all checks (format, vet, tests with race)

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR) dist/
	rm -f coverage.out coverage.html
	go clean -cache

install: build ## Install binary to $GOPATH/bin
	go install $(LDFLAGS) ./cmd/karadul

docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run: ## Run Docker container
	docker run -d --name $(BINARY_NAME) \
		--cap-add NET_ADMIN \
		--cap-add NET_RAW \
		-p 8080:8080 \
		-p 3478:3478/udp \
		$(BINARY_NAME):$(VERSION) server --addr=:8080

docker-compose-up: ## Start with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop docker-compose
	docker-compose down

run-server: build ## Build and run coordination server
	./$(BINARY_NAME) server --addr=:8080

dev-setup: ## Setup development environment
	go mod download
	go mod verify

mod-tidy: ## Tidy go modules
	go mod tidy

update-deps: ## Update dependencies
	go get -u ./...
	go mod tidy

release: ## Create a new release (usage: make release VERSION=v0.1.0)
	./scripts/release.sh $(VERSION)

watch: ## Watch GitHub Actions workflow
	./scripts/watch-release.sh $(VERSION)

setup-homebrew: ## Setup Homebrew tap
	./scripts/setup-homebrew-tap.sh

ci: fmt vet test-race web-lint web-test web-build build ## Run full CI pipeline
	@echo "✅ CI checks passed"

# ==================== WEB UI ====================

web-install: ## Install web dependencies
	cd $(WEB_DIR) && npm install

web-dev: ## Run web development server
	cd $(WEB_DIR) && npm run dev

web-build: ## Build web UI for production
	cd $(WEB_DIR) && npm run build
	@echo "Copying web build to internal/web/dist/..."
	rm -rf internal/web/dist/*
	cp -r $(WEB_DIR)/dist/* internal/web/dist/

web-lint: ## Lint web UI code
	cd $(WEB_DIR) && npm run lint

web-test: ## Run web UI tests
	cd $(WEB_DIR) && npm run test -- --run

web-test-cover: ## Run web UI tests with coverage
	cd $(WEB_DIR) && npm run test -- --run --coverage

web-preview: ## Preview production build
	cd $(WEB_DIR) && npm run preview

build-with-web: web-build build ## Build with embedded web UI

check-all: fmt vet test-race web-lint web-test ## Run all checks (Go + Web UI)
