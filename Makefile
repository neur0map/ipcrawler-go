# IPCrawler Makefile - Automated Go upgrade and tool installation
# Supports: Ubuntu/Debian, Fedora/RedHat, Arch, macOS, and generic Linux
# No PATH modifications - uses symlinks for immediate availability

# Configuration
GO_VERSION ?= 1.24.5
PROJECT_NAME = ipcrawler
BUILD_DIR = build
CACHE_DIR = $(HOME)/.cache/$(PROJECT_NAME)
TEMPLATES_DIR = database/nuclei-templates

# OS and Architecture Detection
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)

# Convert architecture names to Go conventions
ifeq ($(ARCH),x86_64)
    GOARCH := amd64
else ifeq ($(ARCH),aarch64)
    GOARCH := arm64
else ifeq ($(ARCH),armv7l)
    GOARCH := armv7
else
    GOARCH := $(ARCH)
endif

# Platform string for Go downloads
PLATFORM := $(OS)-$(GOARCH)

# Go download URL
GO_DOWNLOAD_URL := https://go.dev/dl/go$(GO_VERSION).$(PLATFORM).tar.gz

# Fixed installation paths - no PATH edits needed
SYSTEM_GO_PATH := /usr/local/go
USER_GO_PATH := $(HOME)/.go
SYSTEM_BIN_PATH := /usr/local/bin

# Determine BIN_PATH once - test write access to system location first
BIN_PATH := $(shell if [ -w "$(SYSTEM_BIN_PATH)" ] || sudo -n true 2>/dev/null; then echo "$(SYSTEM_BIN_PATH)"; else echo "$(HOME)/bin"; fi)

# Mac-specific: always use /usr/local/bin since ~/bin isn't in PATH
ifeq ($(OS),darwin)
    BIN_PATH := /usr/local/bin
endif

# Export GOBIN to ensure go install puts binaries in our chosen directory
export GOBIN := $(BIN_PATH)

# Tools to install with go install
GO_TOOLS := \
    github.com/projectdiscovery/naabu/v2/cmd/naabu@latest

# System packages to install (nmap)
SYSTEM_PACKAGES := nmap

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Default target
.PHONY: all
all: install

# Main installation target - complete setup including tools
.PHONY: install
install:
	@echo "$(BLUE)🚀 IPCrawler Installation$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@# Ensure BIN_PATH directory exists
	@mkdir -p $(BIN_PATH)
	@$(MAKE) check-prerequisites
	@$(MAKE) check-os
	@$(MAKE) install-go
	@$(MAKE) build
	@$(MAKE) install-binary
	@$(MAKE) install-tools
	@echo ""
	@echo "$(GREEN)✅ Installation Complete!$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "Installed tools are available at: $(BLUE)$(BIN_PATH)$(NC)"
	@echo "You can now run: $(BLUE)ipcrawler$(NC), $(BLUE)naabu$(NC), $(BLUE)nmap$(NC)"

# Update target - pull latest code and rebuild
.PHONY: update
update:
	@echo "$(BLUE)🔄 Updating IPCrawler$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(YELLOW)📥 Pulling latest changes...$(NC)"
	@if git pull --ff-only; then \
		echo "$(GREEN)   ✓ Code updated$(NC)"; \
	else \
		echo "$(RED)   ✗ Git pull failed - check for local changes$(NC)"; \
		exit 1; \
	fi
	@$(MAKE) check-prerequisites
	@$(MAKE) build install-binary
	@echo ""
	@echo "$(GREEN)✅ Update Complete!$(NC)"

# Check prerequisites
.PHONY: check-prerequisites
check-prerequisites:
	@echo "$(BLUE)🔍 Checking Prerequisites...$(NC)"
	@MISSING=""
	
	@# Check for curl
	@if ! command -v curl >/dev/null 2>&1; then \
		MISSING="$$MISSING curl"; \
	else \
		echo "$(GREEN)   ✓ curl$(NC)"; \
	fi
	
	@# Check for tar
	@if ! command -v tar >/dev/null 2>&1; then \
		MISSING="$$MISSING tar"; \
	else \
		echo "$(GREEN)   ✓ tar$(NC)"; \
	fi
	
	@# Check for git (only for update target)
	@if [ "$(MAKECMDGOALS)" = "update" ] && ! command -v git >/dev/null 2>&1; then \
		MISSING="$$MISSING git"; \
	elif command -v git >/dev/null 2>&1; then \
		echo "$(GREEN)   ✓ git$(NC)"; \
	fi
	
	@# Report missing tools
	@if [ -n "$$MISSING" ]; then \
		echo ""; \
		echo "$(RED)✗ Missing required tools:$$MISSING$(NC)"; \
		echo ""; \
		echo "$(YELLOW)Installation instructions:$(NC)"; \
		if [ "$(OS)" = "darwin" ]; then \
			echo "   • macOS: $(BLUE)brew install$$MISSING$(NC)"; \
		elif [ -f /etc/debian_version ]; then \
			echo "   • Debian/Ubuntu: $(BLUE)sudo apt-get install$$MISSING$(NC)"; \
		elif [ -f /etc/redhat-release ]; then \
			echo "   • RHEL/Fedora: $(BLUE)sudo yum install$$MISSING$(NC)"; \
		elif [ -f /etc/arch-release ]; then \
			echo "   • Arch Linux: $(BLUE)sudo pacman -S$$MISSING$(NC)"; \
		else \
			echo "   • Please install:$$MISSING"; \
		fi; \
		exit 1; \
	else \
		echo "$(GREEN)   ✓ All prerequisites installed$(NC)"; \
	fi

# OS detection and validation
.PHONY: check-os
check-os:
	@echo ""
	@echo "$(BLUE)🔍 Detecting Operating System...$(NC)"
	@echo "   • OS: $(OS)"
	@echo "   • Architecture: $(ARCH) ($(GOARCH))"
	@if [ "$(OS)" = "linux" ] || [ "$(OS)" = "darwin" ]; then \
		echo "$(GREEN)   ✓ Supported platform$(NC)"; \
	else \
		echo "$(RED)   ✗ Unsupported platform: $(OS)$(NC)"; \
		exit 1; \
	fi

# Go installation/upgrade - idempotent with symlinks
.PHONY: install-go
install-go:
	@echo ""
	@echo "$(BLUE)🔧 Go Installation/Upgrade$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	
	@# Check current Go version
	@SKIP_INSTALL="false"; \
	CURRENT_GO=$$(command -v go 2>/dev/null); \
	if [ -n "$$CURRENT_GO" ]; then \
		CURRENT_VERSION=$$(go version | cut -d' ' -f3 | sed 's/go//'); \
		echo "   • Current Go: $$CURRENT_VERSION at $$CURRENT_GO"; \
		if [ "$$CURRENT_VERSION" = "$(GO_VERSION)" ]; then \
			echo "$(GREEN)   ✓ Go $(GO_VERSION) already installed$(NC)"; \
			SKIP_INSTALL="true"; \
		fi; \
	else \
		echo "   • No Go installation found"; \
	fi; \
	\
	if [ "$$SKIP_INSTALL" = "false" ]; then \
		INSTALL_PATH=""; \
		BIN_PATH=""; \
		NEED_SUDO=""; \
		\
		if [ -w "$(SYSTEM_BIN_PATH)" ] || sudo -n true 2>/dev/null; then \
			INSTALL_PATH="$(SYSTEM_GO_PATH)"; \
			BIN_PATH="$(SYSTEM_BIN_PATH)"; \
			if [ ! -w "$(SYSTEM_BIN_PATH)" ]; then \
				NEED_SUDO="sudo"; \
			fi; \
			echo "   • Mode: System installation"; \
		else \
			INSTALL_PATH="$(USER_GO_PATH)"; \
			BIN_PATH="$(USER_BIN_PATH)"; \
			mkdir -p "$(USER_BIN_PATH)"; \
			echo "   • Mode: User installation"; \
		fi; \
		\
		echo "$(YELLOW)   • Downloading Go $(GO_VERSION)...$(NC)"; \
		mkdir -p "$(CACHE_DIR)"; \
		if ! curl -L --progress-bar "$(GO_DOWNLOAD_URL)" -o "$(CACHE_DIR)/go$(GO_VERSION).tar.gz"; then \
			echo "$(RED)   ✗ Download failed$(NC)"; \
			exit 1; \
		fi; \
		\
		if [ -d "$$INSTALL_PATH" ]; then \
			echo "$(YELLOW)   • Removing old installation...$(NC)"; \
			$$NEED_SUDO rm -rf "$$INSTALL_PATH"; \
		fi; \
		\
		echo "$(YELLOW)   • Installing Go $(GO_VERSION)...$(NC)"; \
		PARENT_DIR=$$(dirname "$$INSTALL_PATH"); \
		$$NEED_SUDO mkdir -p "$$PARENT_DIR"; \
		$$NEED_SUDO tar -C "$$PARENT_DIR" -xzf "$(CACHE_DIR)/go$(GO_VERSION).tar.gz"; \
		\
		if [ "$$(basename $$INSTALL_PATH)" != "go" ]; then \
			$$NEED_SUDO mv "$$PARENT_DIR/go" "$$INSTALL_PATH"; \
		fi; \
		\
		echo "$(YELLOW)   • Creating symlinks...$(NC)"; \
		$$NEED_SUDO ln -sf "$$INSTALL_PATH/bin/go" "$$BIN_PATH/go"; \
		$$NEED_SUDO ln -sf "$$INSTALL_PATH/bin/gofmt" "$$BIN_PATH/gofmt"; \
		\
		echo "$(GREEN)   ✓ Go $(GO_VERSION) installed successfully$(NC)"; \
		echo "   • Symlinks: $$BIN_PATH/go → $$INSTALL_PATH/bin/go"; \
	fi

# Build ipcrawler - clean outdated builds first
.PHONY: build
build:
	@echo ""
	@echo "$(BLUE)🔨 Building IPCrawler$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	
	@# Go should be available via symlinks
	@if ! command -v go >/dev/null 2>&1; then \
		echo "$(RED)   ✗ Go not found - run 'make install-go' first$(NC)"; \
		exit 1; \
	fi
	
	@echo "   • Go version: $$(go version)"
	@echo "   • Cleaning old builds..."
	
	@# Clean any existing binaries (both in build dir and root)
	@rm -f $(BUILD_DIR)/$(PROJECT_NAME) $(PROJECT_NAME)
	@mkdir -p $(BUILD_DIR)
	
	@echo "   • Building $(PROJECT_NAME)..."
	@if go build -o $(BUILD_DIR)/$(PROJECT_NAME) .; then \
		echo "$(GREEN)   ✓ Build successful$(NC)"; \
		echo "   • Binary: $(BUILD_DIR)/$(PROJECT_NAME)"; \
		echo "   • Size: $$(du -h $(BUILD_DIR)/$(PROJECT_NAME) | cut -f1)"; \
	else \
		echo "$(RED)   ✗ Build failed$(NC)"; \
		exit 1; \
	fi

# Install ipcrawler binary - using symlinks only
.PHONY: install-binary
install-binary:
	@echo ""
	@echo "$(BLUE)📦 Installing IPCrawler Binary$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	
	@if [ ! -f "$(BUILD_DIR)/$(PROJECT_NAME)" ]; then \
		echo "$(RED)   ✗ Binary not found. Run 'make build' first$(NC)"; \
		exit 1; \
	fi
	
	@# Determine install location
	@BIN_PATH=""; \
	NEED_SUDO=""; \
	if [ -w "$(SYSTEM_BIN_PATH)" ] || sudo -n true 2>/dev/null; then \
		BIN_PATH="$(SYSTEM_BIN_PATH)"; \
		if [ ! -w "$(SYSTEM_BIN_PATH)" ]; then \
			NEED_SUDO="sudo"; \
		fi; \
		echo "   • Mode: System installation"; \
	else \
		BIN_PATH="$(USER_BIN_PATH)"; \
		mkdir -p "$(USER_BIN_PATH)"; \
		echo "   • Mode: User installation"; \
	fi; \
	\
	FULL_BINARY_PATH="$$(cd $(BUILD_DIR) && pwd)/$(PROJECT_NAME)"; \
	echo "$(YELLOW)   • Creating symlink...$(NC)"; \
	$$NEED_SUDO ln -sf "$$FULL_BINARY_PATH" "$$BIN_PATH/$(PROJECT_NAME)"; \
	\
	echo "$(GREEN)   ✓ IPCrawler installed successfully$(NC)"; \
	echo "   • Symlink: $$BIN_PATH/$(PROJECT_NAME) → $$FULL_BINARY_PATH"

# Install additional tools - Go tools via go install, system tools via package manager
.PHONY: install-tools
install-tools:
	@echo ""
	@echo "$(BLUE)🛠️  Installing Additional Tools$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	
	@# Ensure Go is available
	@if ! command -v go >/dev/null 2>&1; then \
		echo "$(RED)   ✗ Go not found - run 'make install-go' first$(NC)"; \
		exit 1; \
	fi
	
	@echo "$(YELLOW)📦 Installing Go tools to $(BIN_PATH)...$(NC)"
	@echo "   • GOBIN=$(GOBIN)"
	
	@# Install each Go tool with proper permission handling
	@for tool in $(GO_TOOLS); do \
		TOOL_NAME=$$(echo $$tool | sed 's|.*/||; s|@.*||'); \
		echo ""; \
		echo "$(YELLOW)   • Installing $$TOOL_NAME...$(NC)"; \
		if [ ! -w "$(BIN_PATH)" ]; then \
			echo "     System directory detected - using alternative installation..."; \
			TEMP_BIN=$$(mktemp -d); \
			if GOBIN=$$TEMP_BIN go install $$tool; then \
				if sudo mv $$TEMP_BIN/$$TOOL_NAME $(BIN_PATH)/ 2>/dev/null; then \
					echo "$(GREEN)   ✓ $$TOOL_NAME → $(BIN_PATH)/$$TOOL_NAME$(NC)"; \
				else \
					echo "$(RED)   ✗ Failed to move $$TOOL_NAME to $(BIN_PATH)$(NC)"; \
					echo "     Binary available at: $$TEMP_BIN/$$TOOL_NAME"; \
					echo "     Move manually with: sudo mv $$TEMP_BIN/$$TOOL_NAME $(BIN_PATH)/"; \
				fi; \
			else \
				echo "$(RED)   ✗ Failed to build $$TOOL_NAME$(NC)"; \
			fi; \
			rm -rf $$TEMP_BIN 2>/dev/null || true; \
		else \
			if GOBIN=$(GOBIN) go install $$tool; then \
				echo "$(GREEN)   ✓ $$TOOL_NAME → $(BIN_PATH)/$$TOOL_NAME$(NC)"; \
			else \
				echo "$(RED)   ✗ Failed to install $$TOOL_NAME$(NC)"; \
			fi; \
		fi; \
	done
	
	@echo ""
	@echo "$(YELLOW)📦 Installing system packages...$(NC)"
	
	@# Install system packages based on OS
	@for pkg in $(SYSTEM_PACKAGES); do \
		if command -v $$pkg >/dev/null 2>&1; then \
			echo "$(GREEN)   ✓ $$pkg already installed$(NC)"; \
		else \
			echo "$(YELLOW)   • Installing $$pkg...$(NC)"; \
			if [ "$(OS)" = "darwin" ]; then \
				if command -v brew >/dev/null 2>&1; then \
					brew install $$pkg || echo "$(YELLOW)   ⚠ Failed to install $$pkg - install manually$(NC)"; \
				else \
					echo "$(YELLOW)   ⚠ Homebrew not found - install $$pkg manually$(NC)"; \
				fi; \
			elif [ -f /etc/debian_version ]; then \
				if $$NEED_SUDO apt-get install -y $$pkg 2>/dev/null; then \
					echo "$(GREEN)   ✓ $$pkg installed$(NC)"; \
				else \
					echo "$(YELLOW)   ⚠ Failed to install $$pkg - install manually with: sudo apt-get install $$pkg$(NC)"; \
				fi; \
			elif [ -f /etc/redhat-release ]; then \
				if $$NEED_SUDO yum install -y $$pkg 2>/dev/null; then \
					echo "$(GREEN)   ✓ $$pkg installed$(NC)"; \
				else \
					echo "$(YELLOW)   ⚠ Failed to install $$pkg - install manually with: sudo yum install $$pkg$(NC)"; \
				fi; \
			elif [ -f /etc/arch-release ]; then \
				if $$NEED_SUDO pacman -S --noconfirm $$pkg 2>/dev/null; then \
					echo "$(GREEN)   ✓ $$pkg installed$(NC)"; \
				else \
					echo "$(YELLOW)   ⚠ Failed to install $$pkg - install manually with: sudo pacman -S $$pkg$(NC)"; \
				fi; \
			else \
				echo "$(YELLOW)   ⚠ Unknown OS - install $$pkg manually$(NC)"; \
			fi; \
		fi; \
	done
	
	
	@echo ""
	@echo "$(GREEN)   ✓ Tool installation complete$(NC)"


# Clean build artifacts and cache - remove all binaries
.PHONY: clean
clean:
	@echo "$(BLUE)🧹 Cleaning build artifacts and cache$(NC)"
	@echo "   • Removing build directory..."
	@rm -rf $(BUILD_DIR)
	@echo "   • Removing any root-level binaries..."
	@rm -f $(PROJECT_NAME)
	@echo "   • Cleaning Go cache..."
	@rm -rf $(CACHE_DIR)
	@go clean -cache 2>/dev/null || true
	@echo "$(GREEN)   ✓ Clean complete - all outdated builds removed$(NC)"

# Help target
.PHONY: help
help:
	@echo "$(BLUE)IPCrawler Makefile$(NC)"
	@echo "$(BLUE)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@echo "  $(GREEN)make install$(NC)      - Complete installation (Go + IPCrawler + tools)"
	@echo "  $(GREEN)make update$(NC)       - Update to latest code and rebuild"
	@echo "  $(GREEN)make build$(NC)        - Build IPCrawler binary to build/ (cleans old builds)"
	@echo "  $(GREEN)make install-tools$(NC) - Install/update naabu, nmap"
	@echo "  $(GREEN)make clean$(NC)        - Clean all build artifacts and outdated binaries"
	@echo "  $(GREEN)make help$(NC)         - Show this help message"
	@echo ""
	@echo "$(YELLOW)Configuration:$(NC)"
	@echo "  $(BLUE)GO_VERSION$(NC)    - Go version to install (default: $(GO_VERSION))"
	@echo "  $(BLUE)GOBIN$(NC)         - Binary installation path ($(GOBIN))"
	@echo ""
	@echo "$(YELLOW)Tools installed:$(NC)"
	@echo "  • Go tools: naabu (via go install)"
	@echo "  • System tools: nmap (via package manager)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make install                    # First-time setup with all tools"
	@echo "  make update                     # Get latest changes"
	@echo "  make install-tools              # Update tools only"
	@echo ""
	@echo "$(YELLOW)Notes:$(NC)"
	@echo "  • No PATH modifications needed - uses GOBIN=$(GOBIN)"
	@echo "  • All tools installed to: $(BIN_PATH)"
	@echo "  • Commands available immediately after install"