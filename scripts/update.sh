#!/bin/bash

# Comprehensive update script for IPCrawler

set -e

echo "🔄 IPCrawler Update Tool"
echo "======================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if we're in a git repo
if [ ! -d .git ]; then
    echo -e "${RED}❌ Not a git repository!${NC}"
    echo "This appears to be a standalone installation."
    echo ""
    echo "To enable updates, clone the repository:"
    echo "  git clone https://github.com/YOUR_USERNAME/ipcrawler.git"
    echo "  cd ipcrawler"
    echo "  make install"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo -e "${YELLOW}⚠️  You have uncommitted changes:${NC}"
    git status --short
    echo ""
    read -p "Do you want to stash them and continue? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git stash push -m "Auto-stash before update $(date +%Y-%m-%d_%H:%M:%S)"
        echo -e "${GREEN}✅ Changes stashed${NC}"
        STASHED=true
    else
        echo "Update cancelled."
        exit 1
    fi
fi

# Check current branch
CURRENT_BRANCH=$(git branch --show-current)
echo "📍 Current branch: $CURRENT_BRANCH"

# Fetch latest changes
echo "📥 Fetching latest changes..."
git fetch origin || {
    echo -e "${RED}❌ Failed to fetch from origin${NC}"
    echo "Make sure you have set up the remote:"
    echo "  git remote add origin https://github.com/YOUR_USERNAME/ipcrawler.git"
    exit 1
}

# Check if we're behind
LOCAL=$(git rev-parse @)
REMOTE=$(git rev-parse @{u} 2>/dev/null || echo "")
BASE=$(git merge-base @ @{u} 2>/dev/null || echo "")

if [ "$LOCAL" = "$REMOTE" ]; then
    echo -e "${GREEN}✅ Already up to date!${NC}"
elif [ "$LOCAL" = "$BASE" ]; then
    echo "📦 Updates available. Pulling..."
    git pull origin "$CURRENT_BRANCH"
    echo -e "${GREEN}✅ Code updated${NC}"
    NEEDS_BUILD=true
elif [ "$REMOTE" = "$BASE" ]; then
    echo -e "${YELLOW}⚠️  You have local commits not pushed to origin${NC}"
    echo "Your local changes will be preserved."
    NEEDS_BUILD=true
else
    echo -e "${YELLOW}⚠️  Diverged from origin${NC}"
    echo "You may need to merge or rebase manually."
    NEEDS_BUILD=true
fi

# Build if needed
if [ "$NEEDS_BUILD" = true ] || [ ! -f ipcrawler ]; then
    echo "🔨 Building ipcrawler..."
    go build -o ipcrawler || {
        echo -e "${RED}❌ Build failed!${NC}"
        exit 1
    }
    echo -e "${GREEN}✅ Build complete${NC}"
fi

# Update global installation if symlink exists
if [ -L ~/.local/bin/ipcrawler ]; then
    echo -e "${GREEN}✅ Global command updated${NC}"
fi

# Restore stashed changes if any
if [ "$STASHED" = true ]; then
    echo ""
    read -p "Do you want to restore your stashed changes? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git stash pop
        echo -e "${GREEN}✅ Changes restored${NC}"
        echo -e "${YELLOW}Note: You may need to rebuild if your changes affect the code${NC}"
    fi
fi

echo ""
echo -e "${GREEN}🎉 Update complete!${NC}"
ipcrawler --version