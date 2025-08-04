.PHONY: build install dev run clean update help check-go install-go setup-go force-build

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
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 21 ]); then \
			echo "✅ Go version is compatible ($$GO_CURRENT >= 1.21)"; \
		else \
			echo "❌ Go version $$GO_CURRENT is too old (requires >= 1.21)"; \
			echo ""; \
			echo "📦 IPCrawler requires Go 1.21 or later to build properly."; \
			echo "🔧 Would you like to upgrade Go to version $(GO_VERSION)? [y/N]"; \
			read -r UPGRADE_GO; \
			if [ "$$UPGRADE_GO" = "y" ] || [ "$$UPGRADE_GO" = "Y" ] || [ "$$UPGRADE_GO" = "yes" ]; then \
				echo "🚀 Starting Go upgrade..."; \
				$(MAKE) install-go; \
			else \
				echo "⚠️  Build may fail with Go $$GO_CURRENT"; \
				echo "💡 You can upgrade later with: make install-go"; \
			fi; \
		fi; \
	else \
		echo "❌ Go is not installed or not in PATH"; \
		echo ""; \
		echo "📦 IPCrawler requires Go to build."; \
		echo "🔧 Would you like to install Go $(GO_VERSION)? [y/N]"; \
		read -r INSTALL_GO; \
		if [ "$$INSTALL_GO" = "y" ] || [ "$$INSTALL_GO" = "Y" ] || [ "$$INSTALL_GO" = "yes" ]; then \
			echo "🚀 Starting Go installation..."; \
			$(MAKE) install-go; \
		else \
			echo "❌ Cannot build without Go"; \
			echo "💡 Install Go manually or run: make install-go"; \
			exit 1; \
		fi; \
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
	@if command -v go >/dev/null 2>&1; then \
		echo "  GOPATH: $$(go env GOPATH 2>/dev/null || echo '$$HOME/go')"; \
		echo "  GOROOT: $$(go env GOROOT 2>/dev/null || echo 'default')"; \
		echo "✅ Go environment ready!"; \
	else \
		echo "❌ Go still not available after installation"; \
		echo "🔄 Please try one of the following:"; \
		echo "   export PATH=$$PATH:/usr/local/go/bin"; \
		echo "   source ~/.bashrc"; \
		echo "   source ~/.zshrc"; \
		exit 1; \
	fi

# Build the binary (with Go check)
build: setup-go
	@echo "🔨 Building ipcrawler..."
	@if command -v go >/dev/null 2>&1; then \
		go build -o ipcrawler; \
		echo "✅ Build complete!"; \
	else \
		echo "❌ Go not found after setup. This usually means PATH needs to be reloaded."; \
		echo "🔄 Please run one of the following and try again:"; \
		echo "   source ~/.bashrc && make build"; \
		echo "   source ~/.zshrc && make build"; \
		echo "   export PATH=$$PATH:/usr/local/go/bin && make build"; \
		echo "   OR restart your terminal and run 'make build'"; \
		exit 1; \
	fi

# Force build after reloading environment (for when PATH needs refresh)
force-build:
	@echo "🔨 Force building ipcrawler (assuming Go is in PATH)..."
	@export PATH=$$PATH:/usr/local/go/bin && go build -o ipcrawler
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
	@echo "  make             - Build the binary (auto-installs/upgrades Go if needed)"
	@echo "  make install     - Build and install globally"
	@echo "  make update      - Pull latest changes and rebuild"
	@echo "  make dev         - Watch files and auto-rebuild"
	@echo "  make run         - Run without building (use ARGS='...' for arguments)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make check-go    - Check Go installation and version (forces upgrade if < 1.21)"
	@echo "  make install-go  - Install/upgrade Go automatically (Linux/macOS)"
	@echo "  make setup-go    - Setup Go environment"
	@echo "  make force-build - Build using Go in /usr/local/go/bin (after PATH issues)"
	@echo "  make help        - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make                              # Build (installs/upgrades Go if needed)"
	@echo "  make install                      # Install IPCrawler globally"
	@echo "  make check-go                     # Check Go installation"
	@echo "  source ~/.bashrc && make build    # After Go installation on Linux"
	@echo "  make force-build                  # If PATH issues after Go install"
	@echo "  make run ARGS='--version'"
	@echo "  make run ARGS='192.168.1.1 --debug'"
	@echo ""
	@echo "Go Installation:"
	@echo "  - Automatically detects OS (Linux/macOS/Windows)"
	@echo "  - Downloads and installs Go $(GO_VERSION)"
	@echo "  - Forces upgrade if current version < 1.21"
	@echo "  - Sets up PATH and environment variables"
	@echo "  - On Linux: installs to /usr/local/go"
	@echo "  - On macOS: uses Homebrew if available, otherwise .pkg installer"
	@echo ""
	@echo "Troubleshooting:"
	@echo "  If 'go command not found' after installation:"
	@echo "  1. source ~/.bashrc   (or ~/.zshrc)"
	@echo "  2. export PATH=\$$PATH:/usr/local/go/bin"
	@echo "  3. Restart terminal"
	@echo "  4. Try 'make force-build'"