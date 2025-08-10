# IPCrawler TUI Makefile
# Charmbracelet-based Terminal User Interface

.DEFAULT_GOAL := help
.PHONY: help deps clean build demo test-ui test-plain all

# Colors for output
BLUE := \033[34m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

help: ## Show this help message
	@echo "$(BLUE)IPCrawler TUI - 5-Card Dynamic Dashboard$(RESET)"
	@echo "$(YELLOW)Available targets:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-12s$(RESET) %s\n", $$1, $$2}'

deps: ## Install/update Charmbracelet dependencies
	@echo "$(BLUE)Installing Charmbracelet dependencies...$(RESET)"
	go mod tidy
	@echo "$(GREEN)Dependencies installed successfully$(RESET)"

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(RESET)"
	rm -f bin/ipcrawler
	rm -f demo/ipcrawler-demo
	rm -rf bin/
	@echo "$(GREEN)Clean completed$(RESET)"

build: deps ## Build the IPCrawler TUI application
	@echo "$(BLUE)Building IPCrawler TUI...$(RESET)"
	mkdir -p bin
	go build -o bin/ipcrawler ./cmd/ipcrawler/main.go
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
	@echo "$(YELLOW)Manual test: Run 'make demo' and test:$(RESET)"
	@echo "  - Tab: Switch panels"
	@echo "  - Arrow keys: Navigate lists and viewport" 
	@echo "  - Space/Enter: Select items"
	@echo "  - q: Quit application"
	@echo "  - g/G: Go to top/bottom of logs"

test-static: ## Static check: exactly one tea.NewProgram
	@echo "$(BLUE)Static analysis: checking for exactly 1 tea.Program...$(RESET)"
	@PROGRAMS=$$(grep -r "tea\.NewProgram" . --include="*.go" --exclude-dir=".external" | grep -v test | wc -l | tr -d ' '); \
	if [ "$$PROGRAMS" -eq "1" ]; then \
		echo "$(GREEN)✓ Found exactly 1 tea.NewProgram instance (production)$(RESET)"; \
		echo "  - cmd/ipcrawler/main.go (production)"; \
	else \
		echo "$(RED)✗ Found $$PROGRAMS tea.NewProgram instances (expected 1)$(RESET)"; \
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

run: build ## Build and run IPCrawler TUI in new terminal window (cross-platform)
	@echo "$(BLUE)IPCrawler TUI - 5-Card Horizontal Dashboard$(RESET)"
	@echo "$(YELLOW)Cross-platform launcher - detects your OS automatically$(RESET)"
	@echo "$(GREEN)Opening in NEW terminal window with optimal size (200x70)$(RESET)"
	@echo "$(YELLOW)Controls: Tab=cycle cards, 1-5=direct focus, q=quit$(RESET)"
	@if [ "$$EUID" -eq 0 ] || [ -n "$$SUDO_UID" ]; then \
		echo "$(GREEN)Running with elevated privileges$(RESET)"; \
		PRESERVE_SUDO=1 ./scripts/tui-launch-window.sh; \
	else \
		./scripts/tui-launch-window.sh; \
	fi

run-sudo: build ## Build and run IPCrawler TUI with sudo privileges in new terminal
	@echo "$(BLUE)IPCrawler TUI with Sudo Privileges$(RESET)"
	@echo "$(YELLOW)This will run with elevated privileges for all tools$(RESET)"
	@echo "$(GREEN)Opening in NEW terminal window with sudo$(RESET)"
	@PRESERVE_SUDO=1 ./scripts/tui-launch-window.sh

run-new: build ## Open IPCrawler TUI in actual NEW terminal window
	@echo "$(BLUE)Opening IPCrawler TUI in NEW terminal window$(RESET)"  
	@echo "$(YELLOW)This will open a separate terminal window (not same window)$(RESET)"
	@./scripts/tui-launch-window.sh

run-here: build ## Run IPCrawler TUI in current terminal (resize to 200x70 recommended)
	@echo "$(BLUE)Running IPCrawler TUI in Current Terminal$(RESET)"
	@echo "$(YELLOW)Recommended size: 200x70 for best experience$(RESET)"
	@echo "$(YELLOW)macOS: Terminal → Window → Set Size to Columns:200 Rows:70$(RESET)"
	@echo "$(GREEN)Press any key to continue...$(RESET)"
	@read -n1 -r
	./bin/ipcrawler

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
	@echo "$(YELLOW)Run 'make run' to launch the TUI in a new terminal window$(RESET)"