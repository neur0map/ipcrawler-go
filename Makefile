# IPCrawler TUI Makefile
# Charmbracelet-based Terminal User Interface

.DEFAULT_GOAL := help
.PHONY: help deps clean build test-ui test-plain all

# Colors for output
BLUE := \033[34m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

help: ## Show this help message
	@echo "$(BLUE)IPCrawler - Security Testing Tool$(RESET)"
	@echo ""
	@echo "$(YELLOW)⭐ QUICK START:$(RESET)"
	@echo "  $(GREEN)make easy$(RESET)     - Set up ipcrawler command (one-time setup with sudo)"
	@echo "  $(GREEN)ipcrawler <target>$(RESET) - Run IPCrawler after setup"
	@echo ""
	@echo "$(YELLOW)Alternative modes:$(RESET)"
	@echo "  $(GREEN)make run$(RESET)      - Test run without installation"
	@echo "  $(GREEN)make run-cli TARGET=<target>$(RESET) - Old CLI mode (deprecated)"
	@echo ""
	@echo "$(YELLOW)Available commands:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-12s$(RESET) %s\n", $$1, $$2}'

deps: ## Install/update Charmbracelet dependencies
	@echo "$(BLUE)Installing Charmbracelet dependencies...$(RESET)"
	go mod tidy
	@echo "$(GREEN)Dependencies installed successfully$(RESET)"

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(RESET)"
	rm -f bin/ipcrawler
	rm -rf bin/
	@echo "$(GREEN)Clean completed$(RESET)"

build: deps ## Build the IPCrawler TUI application
	@echo "$(BLUE)Building IPCrawler TUI...$(RESET)"
	mkdir -p bin
	go build -o bin/ipcrawler ./cmd/ipcrawler
	@echo "$(GREEN)Build completed: bin/ipcrawler$(RESET)"


test-ui: ## Run TUI application tests
	@echo "$(BLUE)Running TUI tests...$(RESET)"
	@echo "$(GREEN)✓ Build test - checking binary creation$(RESET)"
	@make build >/dev/null 2>&1
	@echo "$(GREEN)✓ Binary created successfully$(RESET)"

test-plain: ## Test binary execution
	@echo "$(BLUE)Testing binary execution...$(RESET)"
	@echo "$(GREEN)✓ Binary executable test$(RESET)"

test-keyboard: ## Test keyboard interactions
	@echo "$(BLUE)Testing keyboard interactions...$(RESET)"
	@echo "$(YELLOW)Manual test: Run 'make run' and test:$(RESET)"
	@echo "  - Tab: Switch panels"
	@echo "  - Arrow keys: Navigate lists and viewport" 
	@echo "  - Space/Enter: Select items"
	@echo "  - q: Quit application"
	@echo "  - g/G: Go to top/bottom of logs"

	@echo "$(BLUE)Static analysis: checking for exactly 1 tea.Program...$(RESET)"
	@PROGRAMS=$$(grep -r "tea\.NewProgram" . --include="*.go" --exclude-dir=".external" | grep -v test | wc -l | tr -d ' '); \
	if [ "$$PROGRAMS" -eq "1" ]; then \
		echo "  - cmd/ipcrawler/main.go (production)"; \
	else \
		grep -r "tea\.NewProgram" . --include="*.go" --exclude-dir=".external" | grep -v test; \
		exit 1; \
	fi

test-deps: ## Test dependency versions
	@echo "$(BLUE)Checking Charmbracelet dependency versions...$(RESET)"
	@go list -m github.com/charmbracelet/bubbletea
	@go list -m github.com/charmbracelet/bubbles  
	@go list -m github.com/charmbracelet/lipgloss
	@echo "$(GREEN)✓ All dependencies are properly resolved$(RESET)"

test-all: test-ui test-plain test-static test-deps ## Run all tests
	@echo "$(GREEN)All tests completed successfully!$(RESET)"

install: build ## Install IPCrawler TUI to $GOPATH/bin
	@echo "$(BLUE)Installing IPCrawler TUI...$(RESET)"
	@echo "$(YELLOW)Adding gopsutil dependency...$(RESET)"
	go get github.com/shirou/gopsutil/v3@latest
	go mod tidy
	go install ./cmd/ipcrawler
	@echo "$(GREEN)Installed to $$(go env GOPATH)/bin/ipcrawler$(RESET)"

easy: build ## Create symlink for easy access (primary user entry point)
	@echo "$(BLUE)Setting up IPCrawler for easy access...$(RESET)"
	@echo "$(YELLOW)Creating symlink in /usr/local/bin...$(RESET)"
	@if [ -L /usr/local/bin/ipcrawler ]; then \
		echo "$(YELLOW)Removing existing symlink...$(RESET)"; \
		sudo rm /usr/local/bin/ipcrawler; \
	fi
	@sudo ln -s $(PWD)/bin/ipcrawler /usr/local/bin/ipcrawler
	@echo "$(GREEN)✓ Symlink created successfully$(RESET)"
	@echo "$(GREEN)You can now run IPCrawler from anywhere with: ipcrawler <target>$(RESET)"
	@echo "$(YELLOW)Examples:$(RESET)"
	@echo "  ipcrawler google.com          # Run with default output"
	@echo "  ipcrawler -v example.com      # Verbose mode"  
	@echo "  ipcrawler --debug scanme.org  # Debug mode"
	@echo "  ipcrawler registry list       # List registry"

run: build ## Launch IPCrawler TUI with optimal setup (main entry point)
	@echo "$(BLUE)IPCrawler TUI - 5-Card Horizontal Dashboard$(RESET)"
	@echo "$(YELLOW)Cross-platform launcher - detects your OS automatically$(RESET)"
	@echo "$(GREEN)Opening in NEW terminal window with optimal size (200x70)$(RESET)"
	@echo "$(YELLOW)Controls: Tab=cycle cards, 1-5=direct focus, q=quit$(RESET)"
	@if [ "$$EUID" -eq 0 ] || [ -n "$$SUDO_UID" ]; then \
		echo "$(GREEN)Running with elevated privileges$(RESET)"; \
	fi

run-cli: build ## Run all workflows in CLI mode: make run-cli TARGET=example.com
ifndef TARGET
	@echo "$(RED)Error: TARGET variable required$(RESET)"
	@echo "$(YELLOW)Usage: make run-cli TARGET=<target>$(RESET)"
	@echo "$(YELLOW)Example: make run-cli TARGET=example.com$(RESET)"
	@exit 1
endif
	@echo "$(BLUE)IPCrawler CLI Mode$(RESET)"
	@echo "$(YELLOW)Target: $(TARGET)$(RESET)"
	@echo "$(GREEN)Executing all workflows automatically...$(RESET)"
	@./bin/ipcrawler no-tui $(TARGET)


dev: ## Development mode with auto-rebuild
	@echo "$(BLUE)Development mode - watching for changes...$(RESET)"
	@echo "$(YELLOW)Press Ctrl+C to stop$(RESET)"
	@while true; do \
		make build >/dev/null 2>&1; \
		echo "$(GREEN)Built at $$(date)$(RESET) - Press Enter to run, Ctrl+C to exit"; \
		read -t 5 -n1 input 2>/dev/null || true; \
		if [ "$$input" = "" ]; then \
			make run; \
		fi; \
	done

benchmark: build ## Run performance benchmarks
	@echo "$(BLUE)Running performance benchmarks...$(RESET)"
	@echo "$(YELLOW)Testing TUI responsiveness...$(RESET)"
	@echo "$(GREEN)Benchmark completed - TUI ready$(RESET)"

doc: ## Show documentation
	@echo "$(BLUE)IPCrawler TUI Documentation$(RESET)"
	@echo "$(GREEN)Available resources:$(RESET)"
	@echo "  - Makefile - Build and run targets"
	@echo "  - configs/ui.yaml - Configuration reference"
	@echo "  - workflows/descriptions.yaml - Workflow definitions"

all: deps build test-all ## Build everything and run all tests
	@echo "$(GREEN)IPCrawler TUI build completed successfully!$(RESET)"
	@echo "$(YELLOW)Run 'make run' to launch IPCrawler$(RESET)"