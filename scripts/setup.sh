#!/bin/bash

echo "🚀 IPCrawler Setup"
echo "=================="

# Build the binary
echo "1️⃣ Building ipcrawler..."
go build -o ipcrawler || { echo "❌ Build failed"; exit 1; }

# Create symlink
echo "2️⃣ Setting up global command..."
mkdir -p ~/.local/bin
ln -sf "$(pwd)/ipcrawler" ~/.local/bin/ipcrawler

# Check if ~/.local/bin is in PATH
if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    echo ""
    echo "⚠️  ~/.local/bin is not in your PATH"
    echo "Add this to your shell config (.bashrc, .zshrc, etc.):"
    echo ""
    echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
fi

# Create convenience aliases
echo "3️⃣ Creating convenience commands..."
PROJECT_DIR="$(pwd)"
cat > ~/.local/bin/ipcrawler-rebuild << EOF
#!/bin/bash
echo "🔄 Rebuilding ipcrawler from $PROJECT_DIR..."
cd "$PROJECT_DIR" || { echo "❌ Project directory not found: $PROJECT_DIR"; exit 1; }
export PATH=/usr/local/go/bin:\$PATH
make build
EOF
chmod +x ~/.local/bin/ipcrawler-rebuild

echo ""
echo "✅ Setup complete!"
echo ""
echo "Available commands:"
echo "  ipcrawler          - Run the tool"
echo "  ipcrawler-rebuild  - Rebuild from anywhere"
echo "  make              - Build (when in project directory)"
echo "  make install      - Build and update global command"
echo "  make dev          - Auto-rebuild on file changes"
echo ""
echo "Try it: ipcrawler --version"