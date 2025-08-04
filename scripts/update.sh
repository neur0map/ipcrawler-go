#!/bin/bash

echo "🔄 Updating IPCrawler..."
echo "==================="

# Pull latest changes
echo "1️⃣ Pulling latest changes from git..."
if git pull origin main; then
    echo "✅ Git pull successful"
else
    echo "❌ Git pull failed"
    exit 1
fi

# Rebuild and install
echo ""
echo "2️⃣ Rebuilding and installing..."
if make install; then
    echo ""
    echo "✅ Update complete!"
    echo ""
    echo "🎉 IPCrawler has been updated and installed globally"
    echo "Try it: ipcrawler --version"
else
    echo "❌ Build failed"
    exit 1
fi