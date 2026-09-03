.PHONY: help build build-linux build-windows build-darwin test clean install-deps fmt vet run-server run-agent

help:
	@echo "GoMeshCentral Build Commands"
	@echo ""
	@echo "Development:"
	@echo "  make install-deps    Install dependencies"
	@echo "  make build           Build server and agent (current OS)"
	@echo "  make test            Run all tests"
	@echo "  make fmt             Format code"
	@echo "  make vet             Run go vet"
	@echo "  make clean           Clean build artifacts"
	@echo ""
	@echo "Cross-Platform:"
	@echo "  make build-linux     Build for Linux x86_64"
	@echo "  make build-windows   Build for Windows x86_64"
	@echo "  make build-darwin    Build for macOS (Intel + ARM)"
	@echo "  make build-all       Build all platforms"
	@echo ""
	@echo "Running:"
	@echo "  make run-server      Run server locally"
	@echo "  make run-agent       Run agent locally"
	@echo ""
	@echo "Web:"
	@echo "  make web-install     Install web dependencies"
	@echo "  make web-build       Build web assets"
	@echo "  make web-dev         Start dev server"

install-deps:
	go mod download
	go mod verify
	cd web && npm ci

build:
	@echo "Building server and agent..."
	go build -o dist/server ./cmd/server
	go build -o dist/agent ./cmd/agent
	@echo "✓ Binaries built in dist/"

build-linux:
	@echo "Building for Linux x86_64..."
	GOOS=linux GOARCH=amd64 go build -o dist/server-linux ./cmd/server
	GOOS=linux GOARCH=amd64 go build -o dist/agent-linux ./cmd/agent

build-windows:
	@echo "Building for Windows x86_64..."
	GOOS=windows GOARCH=amd64 go build -o dist/server-windows.exe ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o dist/agent-windows.exe ./cmd/agent
	@echo "Building Windows MSI installer..."
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& '.\packaging\windows\build-msi.ps1'" || echo "Warning: MSI build skipped (WiX toolset not installed)"

build-darwin:
	@echo "Building for macOS (Intel + ARM)..."
	GOOS=darwin GOARCH=amd64 go build -o dist/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -o dist/agent-darwin-amd64 ./cmd/agent
	GOOS=darwin GOARCH=arm64 go build -o dist/server-darwin-arm64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build -o dist/agent-darwin-arm64 ./cmd/agent

build-all: build-linux build-windows build-darwin
	@echo "✓ All platforms built"

build-msi:
	@echo "Building Windows MSI installer..."
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& '.\packaging\windows\build-msi.ps1'"

.PHONY: build-msi

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests passed"

coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Code formatted"

vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✓ No issues found"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist/*
	rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

run-server:
	@echo "Starting server (http://localhost:8080)..."
	go run ./cmd/server

run-agent:
	@echo "Starting agent..."
	go run ./cmd/agent -server localhost:8080

web-install:
	@echo "Installing web dependencies..."
	cd web && npm ci

web-build:
	@echo "Building web assets..."
	cd web && npm run build

web-dev:
	@echo "Starting web dev server..."
	cd web && npm run dev

dev-setup: install-deps web-install
	@echo "✓ Development environment ready"

release-draft: build-all
	@echo "✓ Release binaries built. Tag and push to create release:"
	@echo "  git tag v1.0.0"
	@echo "  git push origin v1.0.0"

ci-build: fmt vet test build-all
	@echo "✓ CI build passed"
