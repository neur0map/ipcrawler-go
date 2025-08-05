.PHONY: build install dev run clean update help check-go install-go setup-go force-build clean-go install-user-go ensure-go

# Default target
default: build

# OS and Architecture detection
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
GO_VERSION := 1.24.5

# Determine OS and architecture for Go installation
ifeq ($(UNAME_S),Linux)
    OS = linux
    ARCH = amd64
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

# User-level Go install path
GO_URL := https://go.dev/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz
GO_TAR := /tmp/go$(GO_VERSION).tar.gz
GO_DEST := $(HOME)/.go
GO_BIN := $(GO_DEST)/bin/go

# Check if Go is installed and working
check-go:
	@echo "🔍 Checking Go installation..."
	@export PATH=/usr/local/go/bin:$$PATH; \
	if [ -x "/usr/local/go/bin/go" ]; then \
		echo "✅ Go is installed: $$(/usr/local/go/bin/go version)"; \
		echo "  📍 Using Go from: /usr/local/go/bin/go"; \
		GO_CURRENT=$$(/usr/local/go/bin/go version | cut -d' ' -f3 | cut -d'o' -f2); \
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 23 ]); then \
			echo "✅ Go version is compatible ($$GO_CURRENT >= 1.23)"; \
		else \
			echo "❌ Go version $$GO_CURRENT is too old (requires >= 1.23)"; \
			echo ""; \
			echo "📦 IPCrawler requires Go 1.23 or later to build properly."; \
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
	elif command -v go >/dev/null 2>&1; then \
		echo "⚠️  Found system Go: $$(go version)"; \
		echo "  📍 Using Go from: $$(which go)"; \
		GO_CURRENT=$$(go version | cut -d' ' -f3 | cut -d'o' -f2); \
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 23 ]); then \
			echo "✅ Go version is compatible ($$GO_CURRENT >= 1.23)"; \
		else \
			echo "❌ Go version $$GO_CURRENT is too old (requires >= 1.23)"; \
			echo ""; \
			echo "📦 IPCrawler requires Go 1.23 or later to build properly."; \
			echo "💡 Recommend installing Go $(GO_VERSION) to /usr/local/go for better management"; \
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
		echo "🔍 Checking for existing Go installations..."; \
		if command -v go >/dev/null 2>&1; then \
			CURRENT_GOROOT=$$(go env GOROOT 2>/dev/null || echo "unknown"); \
			echo "  Current Go location: $$CURRENT_GOROOT"; \
			if [ "$$CURRENT_GOROOT" != "/usr/local/go" ] && [ -d "$$CURRENT_GOROOT" ]; then \
				echo "⚠️  Found existing Go installation at $$CURRENT_GOROOT"; \
				echo "🗑️  This will be replaced with Go $(GO_VERSION) at /usr/local/go"; \
			fi; \
		fi; \
		echo "📥 Downloading Go $(GO_VERSION)..."; \
		if command -v wget >/dev/null 2>&1; then \
			wget -q "https://golang.org/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz" -O /tmp/go.tar.gz; \
		elif command -v curl >/dev/null 2>&1; then \
			curl -L "https://golang.org/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz" -o /tmp/go.tar.gz; \
		else \
			echo "❌ Neither wget nor curl found. Please install one of them first."; \
			exit 1; \
		fi; \
		echo "🗑️  Removing old Go installation from /usr/local/go..."; \
		if [ -d "/usr/local/go" ]; then sudo rm -rf /usr/local/go; fi; \
		echo "📦 Installing Go $(GO_VERSION) to /usr/local/go..."; \
		sudo tar -C /usr/local -xzf /tmp/go.tar.gz; \
		rm /tmp/go.tar.gz; \
		echo "🔧 Updating PATH configuration..."; \
		echo "  🧹 Cleaning old Go paths from shell configs..."; \
		if [ -f ~/.bashrc ]; then \
			sed -i '/.*go.*bin/d' ~/.bashrc 2>/dev/null || true; \
			sed -i '/.*\/usr\/lib\/go/d' ~/.bashrc 2>/dev/null || true; \
			echo 'export PATH=/usr/local/go/bin:$$PATH' >> ~/.bashrc; \
			echo "  ✅ Updated ~/.bashrc with clean Go PATH"; \
		fi; \
		if [ -f ~/.zshrc ]; then \
			sed -i '/.*go.*bin/d' ~/.zshrc 2>/dev/null || true; \
			sed -i '/.*\/usr\/lib\/go/d' ~/.zshrc 2>/dev/null || true; \
			echo 'export PATH=/usr/local/go/bin:$$PATH' >> ~/.zshrc; \
			echo "  ✅ Updated ~/.zshrc with clean Go PATH"; \
		fi; \
		if [ -f ~/.profile ]; then \
			sed -i '/.*go.*bin/d' ~/.profile 2>/dev/null || true; \
			sed -i '/.*\/usr\/lib\/go/d' ~/.profile 2>/dev/null || true; \
			echo 'export PATH=/usr/local/go/bin:$$PATH' >> ~/.profile; \
			echo "  ✅ Updated ~/.profile with clean Go PATH"; \
		fi; \
		echo ""; \
		echo "✅ Go $(GO_VERSION) installed successfully to /usr/local/go!"; \
		echo "⚡ Temporarily updating PATH for this session..."; \
		export PATH=/usr/local/go/bin:$$PATH; \
		echo "🔄 For permanent PATH update, run:"; \
		echo "   source ~/.bashrc    (for bash users)"; \
		echo "   source ~/.zshrc     (for zsh users)"; \
		echo "   OR restart your terminal session"; \
		echo ""; \
		echo "🧪 Testing new Go installation..."; \
		if /usr/local/go/bin/go version >/dev/null 2>&1; then \
			echo "✅ Go $(GO_VERSION) is working: $$(/usr/local/go/bin/go version)"; \
		else \
			echo "❌ Go installation verification failed"; \
		fi; \
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
	echo "🎉 Installation complete!"

# Automatically ensure Go is installed - tries user-level first, then system-wide
ensure-go:
	@echo "🔍 Ensuring Go $(GO_VERSION) is available..."
	@if [ -x "$(HOME)/.go/bin/go" ]; then \
		GO_CURRENT=$$($(HOME)/.go/bin/go version | cut -d' ' -f3 | cut -d'o' -f2); \
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 23 ]); then \
			echo "✅ User Go $$GO_CURRENT is compatible (>= 1.23)"; \
		else \
			echo "⚠️  User Go $$GO_CURRENT is too old, upgrading to $(GO_VERSION)..."; \
			$(MAKE) install-user-go; \
		fi; \
	elif [ -x "/usr/local/go/bin/go" ]; then \
		GO_CURRENT=$$(/usr/local/go/bin/go version | cut -d' ' -f3 | cut -d'o' -f2); \
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 23 ]); then \
			echo "✅ System Go $$GO_CURRENT is compatible (>= 1.23)"; \
		else \
			echo "⚠️  System Go $$GO_CURRENT is too old, installing user Go $(GO_VERSION)..."; \
			$(MAKE) install-user-go; \
		fi; \
	elif command -v go >/dev/null 2>&1; then \
		GO_CURRENT=$$(go version | cut -d' ' -f3 | cut -d'o' -f2); \
		GO_MAJOR=$$(echo $$GO_CURRENT | cut -d'.' -f1); \
		GO_MINOR=$$(echo $$GO_CURRENT | cut -d'.' -f2); \
		if [ "$$GO_MAJOR" -gt 1 ] || ([ "$$GO_MAJOR" -eq 1 ] && [ "$$GO_MINOR" -ge 23 ]); then \
			echo "✅ PATH Go $$GO_CURRENT is compatible (>= 1.23)"; \
		else \
			echo "⚠️  PATH Go $$GO_CURRENT is too old, installing user Go $(GO_VERSION)..."; \
			$(MAKE) install-user-go; \
		fi; \
	else \
		echo "❌ No Go installation found"; \
		echo "📦 Installing Go $(GO_VERSION) to user directory (no sudo required)..."; \
		$(MAKE) install-user-go; \
	fi

# Install Go to user's home directory (alternative to system-wide installation)
install-user-go:
	@echo "📦 Installing Go $(GO_VERSION) to user directory $(HOME)/.go..."
	@if [ "$(OS)" != "linux" ] && [ "$(OS)" != "darwin" ]; then \
		echo "❌ User-level Go installation currently only supported on Linux and macOS"; \
		echo "💡 Use 'make install-go' for system-wide installation instead"; \
		exit 1; \
	fi
	@# User-level Go install paths
	@GO_URL=https://go.dev/dl/go$(GO_VERSION).$(OS)-$(ARCH).tar.gz; \
	GO_TAR=/tmp/go$(GO_VERSION).tar.gz; \
	GO_DEST=$(HOME)/.go; \
	GO_BIN=$$GO_DEST/bin/go; \
	echo "📥 Downloading Go $(GO_VERSION)..."; \
	if command -v curl >/dev/null 2>&1; then \
		curl -sSL $$GO_URL -o $$GO_TAR; \
	elif command -v wget >/dev/null 2>&1; then \
		wget -q $$GO_URL -O $$GO_TAR; \
	else \
		echo "❌ Neither curl nor wget found. Please install one of them first."; \
		exit 1; \
	fi; \
	echo "🗑️  Removing old user Go installation..."; \
	rm -rf $$GO_DEST; \
	echo "📦 Extracting Go $(GO_VERSION) to $$GO_DEST..."; \
	mkdir -p $(HOME); \
	tar -C $(HOME) -xzf $$GO_TAR; \
	mv $(HOME)/go $$GO_DEST; \
	rm $$GO_TAR; \
	echo "🔧 Updating PATH configuration for user installation..."; \
	if [ -f ~/.bashrc ]; then \
		sed -i '/.*\.go\/bin/d' ~/.bashrc 2>/dev/null || true; \
		echo 'export PATH="$$HOME/.go/bin:$$PATH"' >> ~/.bashrc; \
		echo "  ✅ Updated ~/.bashrc with user Go PATH"; \
	fi; \
	if [ -f ~/.zshrc ]; then \
		sed -i '/.*\.go\/bin/d' ~/.zshrc 2>/dev/null || true; \
		echo 'export PATH="$$HOME/.go/bin:$$PATH"' >> ~/.zshrc; \
		echo "  ✅ Updated ~/.zshrc with user Go PATH"; \
	fi; \
	if [ -f ~/.profile ]; then \
		sed -i '/.*\.go\/bin/d' ~/.profile 2>/dev/null || true; \
		echo 'export PATH="$$HOME/.go/bin:$$PATH"' >> ~/.profile; \
		echo "  ✅ Updated ~/.profile with user Go PATH"; \
	fi; \
	echo ""; \
	echo "✅ User-level Go $(GO_VERSION) installed successfully to $$GO_DEST!"; \
	echo "🧪 Testing user Go installation..."; \
	if [ -x "$$GO_BIN" ]; then \
		echo "✅ Go installed: $$($$GO_BIN version)"; \
		echo "📍 Location: $$GO_DEST"; \
		echo "⚡ Temporarily updating PATH for this session..."; \
		export PATH="$$GO_DEST/bin:$$PATH"; \
	else \
		echo "❌ User Go installation verification failed"; \
		exit 1; \
	fi; \
	echo "🔄 For permanent PATH update, run:"; \
	echo "   source ~/.bashrc    (for bash users)"; \
	echo "   source ~/.zshrc     (for zsh users)"; \
	echo "   OR restart your terminal session"; \
	echo ""; \
	echo "💡 Note: User installation ($$GO_DEST) takes precedence over system Go"; \
	echo "🎉 Installation complete!"

# Setup Go environment (run after installing Go)
setup-go: check-go
	@echo "🔧 Setting up Go environment..."
	@export PATH=/usr/local/go/bin:$$PATH; \
	echo "  🔍 PATH verification:"; \
	echo "    Current PATH priority: $$(echo $$PATH | tr ':' '\n' | grep -E '(go|bin)' | head -3 | tr '\n' ':' | sed 's/:$$//')"; \
	if [ -x "/usr/local/go/bin/go" ]; then \
		echo "  📍 Using Go from: /usr/local/go/bin/go"; \
		echo "  📦 GOPATH: $$(/usr/local/go/bin/go env GOPATH 2>/dev/null || echo '$$HOME/go')"; \
		echo "  🏠 GOROOT: $$(/usr/local/go/bin/go env GOROOT 2>/dev/null || echo '/usr/local/go')"; \
		echo "  🏷️  Version: $$(/usr/local/go/bin/go version)"; \
		if command -v go >/dev/null 2>&1 && [ "$$(which go)" != "/usr/local/go/bin/go" ]; then \
			echo "  ⚠️  Warning: System Go found at $$(which go)"; \
			echo "      This may cause conflicts. Consider running 'make clean-go'"; \
		fi; \
		echo "✅ Go environment ready!"; \
	elif command -v go >/dev/null 2>&1; then \
		echo "  📍 Using system Go: $$(which go)"; \
		echo "  📦 GOPATH: $$(go env GOPATH 2>/dev/null || echo '$$HOME/go')"; \
		echo "  🏠 GOROOT: $$(go env GOROOT 2>/dev/null || echo 'default')"; \
		echo "  🏷️  Version: $$(go version)"; \
		echo "  💡 Consider installing Go to /usr/local/go for better management"; \
		echo "✅ Go environment ready!"; \
	else \
		echo "❌ Go still not available after installation"; \
		echo "🔄 Please try one of the following:"; \
		echo "   export PATH=/usr/local/go/bin:$$PATH"; \
		echo "   source ~/.bashrc"; \
		echo "   source ~/.zshrc"; \
		echo "   OR restart your terminal"; \
		exit 1; \
	fi

# Build the binary (with automatic Go installation)
build: ensure-go
	@echo "🔨 Building ipcrawler..."
	@export PATH=$(HOME)/.go/bin:/usr/local/go/bin:$$PATH; \
	if [ -x "$(HOME)/.go/bin/go" ]; then \
		echo "  Using user Go: $$($(HOME)/.go/bin/go version)"; \
		echo "  📍 Location: $(HOME)/.go/bin/go"; \
		echo "  📝 Updating go.mod..."; \
		$(HOME)/.go/bin/go mod tidy; \
		$(HOME)/.go/bin/go build -o ipcrawler; \
		echo "✅ Build complete!"; \
	elif [ -x "/usr/local/go/bin/go" ]; then \
		echo "  Using system Go: $$(/usr/local/go/bin/go version)"; \
		echo "  📍 Location: /usr/local/go/bin/go"; \
		echo "  📝 Updating go.mod..."; \
		/usr/local/go/bin/go mod tidy; \
		/usr/local/go/bin/go build -o ipcrawler; \
		echo "✅ Build complete!"; \
	elif command -v go >/dev/null 2>&1; then \
		echo "  Using PATH Go: $$(go version)"; \
		echo "  📍 Location: $$(which go)"; \
		echo "  📝 Updating go.mod..."; \
		go mod tidy; \
		go build -o ipcrawler; \
		echo "✅ Build complete!"; \
	else \
		echo "❌ Go not found after setup. This usually means PATH needs to be reloaded."; \
		echo "🔄 Please run one of the following and try again:"; \
		echo "   source ~/.bashrc && make build"; \
		echo "   source ~/.zshrc && make build"; \
		echo "   export PATH=$(HOME)/.go/bin:/usr/local/go/bin:$$PATH && make build"; \
		echo "   OR restart your terminal and run 'make build'"; \
		exit 1; \
	fi

# Force build after reloading environment (for when PATH needs refresh)
force-build:
	@echo "🔨 Force building ipcrawler with latest Go..."
	@export PATH=$(HOME)/.go/bin:/usr/local/go/bin:$$PATH; \
	if [ -x "$(HOME)/.go/bin/go" ]; then \
		echo "  Using user Go: $$($(HOME)/.go/bin/go version)"; \
		echo "  📝 Updating go.mod..."; \
		$(HOME)/.go/bin/go mod tidy; \
		$(HOME)/.go/bin/go build -o ipcrawler; \
	elif [ -x "/usr/local/go/bin/go" ]; then \
		echo "  Using system Go: $$(/usr/local/go/bin/go version)"; \
		echo "  📝 Updating go.mod..."; \
		/usr/local/go/bin/go mod tidy; \
		/usr/local/go/bin/go build -o ipcrawler; \
	else \
		echo "  Fallback to PATH Go"; \
		echo "  📝 Updating go.mod..."; \
		go mod tidy; \
		go build -o ipcrawler; \
	fi
	@echo "✅ Build complete!"

# Clean up old Go installations (use with caution)
clean-go:
	@echo "🗑️  Cleaning up old Go installations..."
	@echo "⚠️  This will remove system-installed Go packages and old installations"
	@echo "🔧 Would you like to proceed? This will:"
	@echo "   1. Remove system Go packages (apt/yum installed)"
	@echo "   2. Keep only /usr/local/go (our managed installation)"
	@echo "   3. Update PATH to prioritize /usr/local/go/bin"
	@echo ""
	@echo "Proceed with Go cleanup? [y/N]"
	@read -r CLEAN_GO; \
	if [ "$$CLEAN_GO" = "y" ] || [ "$$CLEAN_GO" = "Y" ] || [ "$$CLEAN_GO" = "yes" ]; then \
		echo "🧹 Starting Go cleanup..."; \
		if command -v apt >/dev/null 2>&1; then \
			echo "  Removing Go via apt..."; \
			sudo apt remove -y golang-go golang || true; \
		fi; \
		if command -v yum >/dev/null 2>&1; then \
			echo "  Removing Go via yum..."; \
			sudo yum remove -y golang || true; \
		fi; \
		if command -v dnf >/dev/null 2>&1; then \
			echo "  Removing Go via dnf..."; \
			sudo dnf remove -y golang || true; \
		fi; \
		echo "  Updating PATH priority in shell configs..."; \
		sed -i 's|export PATH=.*go.*|export PATH=/usr/local/go/bin:$$PATH|g' ~/.bashrc 2>/dev/null || true; \
		if [ -f ~/.zshrc ]; then \
			sed -i 's|export PATH=.*go.*|export PATH=/usr/local/go/bin:$$PATH|g' ~/.zshrc 2>/dev/null || true; \
		fi; \
		echo "✅ Go cleanup complete!"; \
		echo "🔄 Please run: source ~/.bashrc (or restart terminal)"; \
		echo "🧪 Then test with: make check-go"; \
	else \
		echo "❌ Go cleanup cancelled"; \
	fi

# Install globally (creates symlink if needed) - handles Go installation automatically
install: ensure-go build
	@echo "🧹 Cleaning Go module cache to prevent version conflicts..."
	@export PATH=$(HOME)/.go/bin:/usr/local/go/bin:$$PATH; \
	if [ -x "$(HOME)/.go/bin/go" ]; then \
		$(HOME)/.go/bin/go clean -modcache 2>/dev/null || true; \
		echo "  Using user Go: $$($(HOME)/.go/bin/go version)"; \
		export GOROOT=$(HOME)/.go; \
	elif [ -x "/usr/local/go/bin/go" ]; then \
		/usr/local/go/bin/go clean -modcache 2>/dev/null || true; \
		echo "  Using system Go: $$(/usr/local/go/bin/go version)"; \
		export GOROOT=/usr/local/go; \
	elif command -v go >/dev/null 2>&1; then \
		go clean -modcache 2>/dev/null || true; \
		echo "  Using PATH Go: $$(go version)"; \
		export GOROOT=$$(go env GOROOT); \
	fi
	@echo "🔧 Running setup script with correct Go environment..."
	@export PATH=$(HOME)/.go/bin:/usr/local/go/bin:$$PATH GOPATH=$$HOME/go; ./scripts/setup.sh

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
	@echo "🔨 Rebuilding and installing globally..."
	@$(MAKE) install
	@echo "✅ Update complete! IPCrawler is now up to date."
	@echo "🎉 Global command updated - try: ipcrawler --version"

# Show help
help:
	@echo "IPCrawler Build Commands:"
	@echo "  make             - Build the binary (auto-installs Go if needed)"
	@echo "  make install     - Build and install globally (auto-installs Go if needed)"
	@echo "  make update      - Pull latest changes, rebuild, and update global command"
	@echo "  make dev         - Watch files and auto-rebuild"
	@echo "  make run         - Run without building (use ARGS='...' for arguments)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make check-go      - Check Go installation and version (forces upgrade if < 1.23)"
	@echo "  make install-go    - Install/upgrade Go system-wide (requires sudo)"
	@echo "  make install-user-go - Install Go to user directory (~/.go) - no sudo needed"
	@echo "  make setup-go      - Setup Go environment"
	@echo "  make force-build - Build using Go in /usr/local/go/bin (after PATH issues)"
	@echo "  make clean-go    - Remove old Go installations (keeps only /usr/local/go)"
	@echo "  make help        - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make                              # Build (auto-installs Go if needed)"
	@echo "  make install                      # Install IPCrawler globally (auto-installs Go)"
	@echo "  make check-go                     # Check Go installation"
	@echo "  source ~/.bashrc && make build    # After Go installation on Linux"
	@echo "  make force-build                  # If PATH issues after Go install"
	@echo "  make clean-go                     # Remove old Go versions"
	@echo "  make run ARGS='--version'"
	@echo "  make run ARGS='192.168.1.1 --debug'"
	@echo ""
	@echo "Automatic Go Installation:"
	@echo "  - 'make' and 'make install' automatically handle Go installation"
	@echo "  - No separate commands needed - just run 'make install'!"
	@echo "  - Automatically detects OS (Linux/macOS/Windows)"
	@echo "  - Downloads and installs Go $(GO_VERSION) if needed"
	@echo "  - Upgrades automatically if current version < 1.23"
	@echo "  - Prefers user-level installation (no sudo required)"
	@echo "  - Sets up PATH and environment variables"
	@echo ""
	@echo "  Installation Priority:"
	@echo "    1. User-level: ~/.go (preferred, no sudo needed)"
	@echo "    2. System-wide: /usr/local/go (fallback if user install fails)"
	@echo "    3. Uses existing compatible Go if found"
	@echo ""
	@echo "  Manual Installation (if needed):"
	@echo "    make install-user-go    # Install to ~/.go (no sudo)"
	@echo "    make install-go         # Install system-wide (requires sudo)"
	@echo ""
	@echo "Troubleshooting:"
	@echo "  If 'go command not found' after installation:"
	@echo "  1. source ~/.bashrc   (or ~/.zshrc)"
	@echo "  2. export PATH=/usr/local/go/bin:\$$PATH"
	@echo "  3. make force-build   (uses /usr/local/go/bin directly)"
	@echo "  4. make clean-go      (removes conflicting Go installations)"
	@echo "  5. Restart terminal"