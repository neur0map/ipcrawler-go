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
echo "🎯 Creating PATH activation script..."

# Create an activation script that the user can source
cat > "$HOME/.go_activate" << 'EOF'
export PATH="$HOME/.go/bin:$PATH"
EOF

echo ""
echo "🎉 Installation complete!"
echo ""
echo "🚨 IMPORTANT: To activate Go 1.24.5 in your current terminal, run:"
echo ""
echo "    source ~/.go_activate"
echo ""
echo "🧪 Then verify with: go version"
echo ""
echo "📝 Future terminal sessions will automatically use Go 1.24.5"
echo "🏃 Try: ipcrawler --version"