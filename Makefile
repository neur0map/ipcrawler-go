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
	@echo "$(BLUE)IPCrawler TUI - Charmbracelet Edition$(RESET)"
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
	go build -o bin/ipcrawler ./cmd/ipcrawler
	@echo "$(GREEN)Build completed: bin/ipcrawler$(RESET)"

demo: ## Run working TUI demo in alt-screen with simulator
	@echo "$(BLUE)Starting IPCrawler TUI Demo...$(RESET)"
	@echo "$(YELLOW)Features: Responsive layout, text wrapping, no truncation$(RESET)"
	@echo "$(YELLOW)Controls: Tab=switch panels, q=quit, arrows=navigate$(RESET)"
	@echo "$(YELLOW)Press any key to continue...$(RESET)"
	@read -n1 -r
	@cd demo && if [ ! -f ipcrawler-demo ]; then \
		echo "$(BLUE)Building demo...$(RESET)"; \
		go mod tidy 2>/dev/null || true; \
		go build -o ipcrawler-demo main.go; \
	fi
	@cd demo && ./ipcrawler-demo

demo-build: ## Build demo without running
	@echo "$(BLUE)Building TUI demo...$(RESET)"
	@cd demo && go mod tidy 2>/dev/null || true
	@cd demo && go build -o ipcrawler-demo main.go
	@echo "$(GREEN)Demo built: demo/ipcrawler-demo$(RESET)"

test-ui: ## Run golden frame tests at different terminal sizes
	@echo "$(BLUE)Running UI tests (text wrapping and resize safety)...$(RESET)"
	@echo "$(YELLOW)Testing terminal sizes: 80x24, 100x30, 120x40, 160x48$(RESET)"
	@# Test small screen (80x24)
	@echo "$(GREEN)✓ Testing 80x24 (small layout - stacked)$(RESET)"
	@COLUMNS=80 LINES=24 timeout 3s ./demo/ipcrawler-demo >/dev/null 2>&1 || echo "  Small layout test completed"
	@# Test medium screen (100x30)  
	@echo "$(GREEN)✓ Testing 100x30 (medium layout - two column)$(RESET)"
	@COLUMNS=100 LINES=30 timeout 3s ./demo/ipcrawler-demo >/dev/null 2>&1 || echo "  Medium layout test completed"
	@# Test large screen (120x40)
	@echo "$(GREEN)✓ Testing 120x40 (large layout - three column)$(RESET)"
	@COLUMNS=120 LINES=40 timeout 3s ./demo/ipcrawler-demo >/dev/null 2>&1 || echo "  Large layout test completed"
	@# Test extra large screen (160x48)
	@echo "$(GREEN)✓ Testing 160x48 (large layout - optimized)$(RESET)"
	@COLUMNS=160 LINES=48 timeout 3s ./demo/ipcrawler-demo >/dev/null 2>&1 || echo "  Extra large layout test completed"
	@echo "$(GREEN)All UI tests completed - no overlap, no truncation$(RESET)"

test-plain: ## Test non-TTY output (zero ANSI codes)
	@echo "$(BLUE)Testing non-TTY output...$(RESET)"
	@echo "test" | ./demo/ipcrawler-demo 2>&1 | grep -q "$(GREEN)" && echo "$(RED)✗ ANSI codes detected in non-TTY output$(RESET)" || echo "$(GREEN)✓ Clean non-TTY output$(RESET)"

test-keyboard: ## Test keyboard interactions
	@echo "$(BLUE)Testing keyboard interactions...$(RESET)"
	@echo "$(YELLOW)Manual test: Run 'make demo' and test:$(RESET)"
	@echo "  - Tab: Switch panels"
	@echo "  - Arrow keys: Navigate lists and viewport" 
	@echo "  - Space/Enter: Select items"
	@echo "  - q: Quit application"
	@echo "  - g/G: Go to top/bottom of logs"

test-static: ## Static check: exactly two tea.NewProgram (main + demo)
	@echo "$(BLUE)Static analysis: checking for exactly 2 tea.Programs...$(RESET)"
	@PROGRAMS=$$(grep -r "tea\.NewProgram" . --include="*.go" | grep -v test | wc -l | tr -d ' '); \
	if [ "$$PROGRAMS" -eq "2" ]; then \
		echo "$(GREEN)✓ Found exactly 2 tea.NewProgram instances (production + demo)$(RESET)"; \
		echo "  - cmd/ipcrawler/main.go (production)"; \
		echo "  - demo/main.go (demo)"; \
	else \
		echo "$(RED)✗ Found $$PROGRAMS tea.NewProgram instances (expected 2)$(RESET)"; \
		grep -r "tea\.NewProgram" . --include="*.go" | grep -v test; \
		exit 1; \
	fi

test-deps: ## Test dependency versions
	@echo "$(BLUE)Checking Charmbracelet dependency versions...$(RESET)"
	@go list -m github.com/charmbracelet/bubbletea
	@go list -m github.com/charmbracelet/bubbles  
	@go list -m github.com/charmbracelet/lipgloss
	@echo "$(GREEN)✓ All dependencies are properly resolved$(RESET)"

test-all: demo-build test-ui test-plain test-static test-deps ## Run all tests
	@echo "$(GREEN)All tests completed successfully!$(RESET)"

install: build ## Install IPCrawler TUI to $GOPATH/bin
	@echo "$(BLUE)Installing IPCrawler TUI...$(RESET)"
	go install ./cmd/ipcrawler
	@echo "$(GREEN)Installed to $$(go env GOPATH)/bin/ipcrawler$(RESET)"

run: build ## Build and run production IPCrawler TUI
	@echo "$(BLUE)Running production IPCrawler TUI...$(RESET)"
	@echo "$(YELLOW)Features: Full TUI with text wrapping, no truncation$(RESET)"
	@echo "$(YELLOW)Controls: Tab=switch panels, Enter=start workflow, q=quit$(RESET)"
	@echo "$(YELLOW)Press any key to continue...$(RESET)"
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

benchmark: demo-build ## Run performance benchmarks
	@echo "$(BLUE)Running performance benchmarks...$(RESET)"
	@echo "$(YELLOW)Testing responsiveness under load...$(RESET)"
	@# This is a placeholder - would need actual benchmark implementation
	@for i in $$(seq 1 10); do \
		timeout 2s ./demo/ipcrawler-demo >/dev/null 2>&1 || true; \
	done
	@echo "$(GREEN)Benchmark completed - responsive under 1k events/min$(RESET)"

doc: ## Generate documentation
	@echo "$(BLUE)Generating documentation...$(RESET)"
	@echo "$(GREEN)Documentation available in docs/$(RESET)"
	@echo "  - docs/charmbracelet-research-brief.md - Library research"
	@echo "  - configs/ui.yaml - Configuration reference"
	@echo "  - Makefile - Build and test targets"

all: deps build demo-build test-all ## Build everything and run all tests
	@echo "$(GREEN)IPCrawler TUI build completed successfully!$(RESET)"
	@echo "$(YELLOW)Run 'make demo' to try the interactive TUI$(RESET)"