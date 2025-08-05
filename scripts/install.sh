#!/bin/bash
set -e

echo "🚀 IPCrawler Installation Script"
echo "================================"

# Get the project root directory (parent of scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Ensure Go is installed
echo "🔍 Ensuring Go 1.24.5 is available..."
make ensure-go

# Build the project
echo "🔨 Building IPCrawler..."
make build

# Run the setup script
echo "🔧 Running setup script..."
export PATH="$HOME/.go/bin:/usr/local/go/bin:$PATH"
export GOPATH="$HOME/go"

if [ -x "$HOME/.go/bin/go" ]; then
    $HOME/.go/bin/go clean -modcache 2>/dev/null || true
    echo "  Using user Go: $($HOME/.go/bin/go version)"
    export GOROOT="$HOME/.go"
elif [ -x "/usr/local/go/bin/go" ]; then
    /usr/local/go/bin/go clean -modcache 2>/dev/null || true
    echo "  Using system Go: $(/usr/local/go/bin/go version)"
    export GOROOT="/usr/local/go"
elif command -v go >/dev/null 2>&1; then
    go clean -modcache 2>/dev/null || true
    echo "  Using PATH Go: $(go version)"
    export GOROOT="$(go env GOROOT)"
fi

./scripts/setup.sh

echo ""
echo "🎯 Activating Go 1.24.5 in current session..."

# Update PATH for current session
export PATH="$HOME/.go/bin:$PATH"

echo ""
echo "🧪 Testing Go version:"
if command -v go >/dev/null 2>&1; then
    go version
    if go version | grep -q "go1.24.5"; then
        echo "✅ SUCCESS! Go 1.24.5 is now active!"
    else
        echo "⚠️  Warning: Expected Go 1.24.5, but got: $(go version)"
        echo "💡 You may need to restart your terminal or run:"
        echo "    export PATH=\"\$HOME/.go/bin:\$PATH\""
    fi
else
    echo "❌ Go not found. Please restart your terminal or run:"
    echo "    export PATH=\"\$HOME/.go/bin:\$PATH\""
fi

echo ""
echo "🎉 Installation complete!"
echo ""
echo "📝 To use Go 1.24.5 in future terminal sessions, it's been added to your shell config."
echo "📝 For immediate use in THIS session, the PATH has been updated."
echo ""
echo "🏃 Try: ipcrawler --version"