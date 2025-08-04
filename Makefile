.PHONY: build install dev run clean update help check-go install-go setup-go

# Default target
default: build

# OS and Architecture detection
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
GO_VERSION := 1.21.5

# Determine OS and architecture for Go installation
ifeq ($(UNAME_S),Linux)
    OS = linux
    ifeq ($(UNAME_M),x86_64)
        ARCH = amd64
    else ifeq ($(UNAME_M),aarch64)
        ARCH = arm64
    else ifeq ($(UNAME_M),armv7l)
        ARCH = armv6l
    else
        ARCH = amd64
    endif
endif
ifeq ($(UNAME_S),Darwin)
    OS = darwin
    ifeq ($(UNAME_M),arm64)
        ARCH = arm64
    else
        ARCH = amd64
    endif
endif
ifeq ($(OS),Windows_NT)
    OS = windows
    ARCH = amd64
endif

# Check if Go is installed and working
check-go:
	@echo "🔍 Checking Go installation..."
	@if command -v go >/dev/null 2>&1; then \
		echo "✅ Go is installed: $$(go version)"; \
		GO_CURRENT=$$(go version | cut -d' ' -f3 | cut -d'o' -f2); \
		if printf '%s\n%s\n' "1.21" "$$GO_CURRENT" | sort -V | head -n1 | grep -q "1.21"; then \
			echo "✅ Go version is compatible ($$GO_CURRENT >= 1.21)"; \
		else \
			echo "⚠️  Go version $$GO_CURRENT might be too old, recommended: 1.21+"; \
		fi; \
	else \
		echo "❌ Go is not installed or not in PATH"; \
		echo "🔧 Running Go installation..."; \
		$(MAKE) install-go; \
	fi

# Install Go automatically based on OS
install-go:
	@echo "📦 Installing Go $(GO_VERSION) for $(OS)/$(ARCH)..."
	@if [ "$(OS)" = "linux" ]; then \
		echo "🐧 Installing Go on Linux..."; \
		if command -v wget >/dev/null 2>&1; then \
			wget -q "https://golang.org/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz" -O /tmp/go.tar.gz; \
		elif command -v curl >/dev/null 2>&1; then \
			curl -L "https://golang.org/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz" -o /tmp/go.tar.gz; \
		else \
			echo "❌ Neither wget nor curl found. Please install one of them first."; \
			exit 1; \
		fi; \
		if [ -d "/usr/local/go" ]; then sudo rm -rf /usr/local/go; fi; \
		sudo tar -C /usr/local -xzf /tmp/go.tar.gz; \
		rm /tmp/go.tar.gz; \
		if ! grep -q "/usr/local/go/bin" ~/.bashrc 2>/dev/null; then \
			echo 'export PATH=$$PATH:/usr/local/go/bin' >> ~/.bashrc; \
		fi; \
		if [ -f ~/.zshrc ] && ! grep -q "/usr/local/go/bin" ~/.zshrc; then \
			echo 'export PATH=$$PATH:/usr/local/go/bin' >> ~/.zshrc; \
		fi; \
		echo "✅ Go installed successfully!"; \
		echo "🔄 Please run one of the following to update your PATH:"; \
		echo "   source ~/.bashrc    (for bash users)"; \
		echo "   source ~/.zshrc     (for zsh users)"; \
		echo "   OR restart your terminal session"; \
		echo ""; \
		echo "🧪 Then test with: go version"; \
	elif [ "$(OS)" = "darwin" ]; then \
		echo "🍎 Installing Go on macOS..."; \
		if command -v brew >/dev/null 2>&1; then \
			echo "🍺 Using Homebrew to install Go..."; \
			brew install go; \
		else \
			echo "📥 Downloading Go installer package..."; \
			curl -L "https://golang.org/dl/go$(GO_VERSION).$(OS)-$(ARCH).pkg" -o /tmp/go.pkg; \
			echo "📦 Installing Go package..."; \
			sudo installer -pkg /tmp/go.pkg -target /; \
			rm /tmp/go.pkg; \
		fi; \
		echo "✅ Go installed successfully!"; \
	else \
		echo "❌ Automatic Go installation not supported for $(OS)"; \
		echo "💡 Please install Go manually from: https://golang.org/dl/"; \
		echo "   Recommended version: $(GO_VERSION) or later"; \
		echo "   Download: go$(GO_VERSION).$(OS)-$(ARCH).tar.gz"; \
		exit 1; \
	fi; \
	echo ""; \
	echo "🎉 Installation complete! Run 'make check-go' to verify."

# Setup Go environment (run after installing Go)
setup-go: check-go
	@echo "🔧 Setting up Go environment..."
	@go env GOPATH >/dev/null 2>&1 || export GOPATH=$$HOME/go
	@echo "✅ Go environment ready!"

# Build the binary (with Go check)
build: setup-go
	@echo "🔨 Building ipcrawler..."
	@go build -o ipcrawler
	@echo "✅ Build complete!"

# Install globally (creates symlink if needed)
install: build
	@./scripts/setup.sh

# Development mode - auto-rebuild on file changes (requires watchexec)
dev:
	@if command -v watchexec > /dev/null; then \
		echo "👀 Watching for changes..."; \
		watchexec -e go -r "make build && echo '✅ Rebuilt!' || echo '❌ Build failed'"; \
	else \
		echo "❌ watchexec not found. Install with: brew install watchexec"; \
		exit 1; \
	fi

# Run directly without building
run:
	@go run main.go $(ARGS)

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -f ipcrawler
	@echo "✅ Clean complete!"

# Update from git and rebuild
update:
	@echo "🔄 Updating IPCrawler..."
	@echo "📥 Pulling latest changes..."
	@git pull origin main || { \
		echo "❌ Git pull failed!"; \
		echo "💡 If this is your first time, set up the remote:"; \
		echo "   git remote add origin https://github.com/YOUR_USERNAME/ipcrawler.git"; \
		exit 1; \
	}
	@echo "🔨 Rebuilding..."
	@$(MAKE) build
	@echo "✅ Update complete! IPCrawler is now up to date."

# Show help
help:
	@echo "IPCrawler Build Commands:"
	@echo "  make             - Build the binary (auto-installs Go if needed)"
	@echo "  make install     - Build and install globally"
	@echo "  make update      - Pull latest changes and rebuild"
	@echo "  make dev         - Watch files and auto-rebuild"
	@echo "  make run         - Run without building (use ARGS='...' for arguments)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make check-go    - Check Go installation and version"
	@echo "  make install-go  - Install Go automatically (Linux/macOS)"
	@echo "  make setup-go    - Setup Go environment"
	@echo "  make help        - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make             # Build (installs Go if missing)"
	@echo "  make install     # Install IPCrawler globally"
	@echo "  make check-go    # Check Go installation"
	@echo "  make update      # Update from git and rebuild"
	@echo "  make run ARGS='--version'"
	@echo "  make run ARGS='192.168.1.1 --debug'"
	@echo ""
	@echo "Go Installation:"
	@echo "  - Automatically detects OS (Linux/macOS/Windows)"
	@echo "  - Downloads and installs Go $(GO_VERSION)"
	@echo "  - Sets up PATH and environment variables"
	@echo "  - On Linux: installs to /usr/local/go"
	@echo "  - On macOS: uses Homebrew if available, otherwise .pkg installer"