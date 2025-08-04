.PHONY: build install dev run clean update help

# Default target
default: build

# Build the binary
build:
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
	@echo "  make         - Build the binary"
	@echo "  make install - Build and install globally"
	@echo "  make update  - Pull latest changes and rebuild"
	@echo "  make dev     - Watch files and auto-rebuild"
	@echo "  make run     - Run without building (use ARGS='...' for arguments)"
	@echo "  make clean   - Remove build artifacts"
	@echo "  make help    - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make"
	@echo "  make install"
	@echo "  make update"
	@echo "  make run ARGS='--version'"
	@echo "  make run ARGS='192.168.1.1 --debug'"