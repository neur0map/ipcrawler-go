#!/bin/bash

# Smart build script that only rebuilds if source files changed

BINARY="ipcrawler"
SOURCE_FILES=$(find . -name "*.go" -o -name "go.mod" | grep -v ".git")

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo "🔨 Binary not found. Building..."
    go build -o "$BINARY"
    exit $?
fi

# Check if any source file is newer than the binary
REBUILD_NEEDED=false
for file in $SOURCE_FILES; do
    if [ "$file" -nt "$BINARY" ]; then
        REBUILD_NEEDED=true
        break
    fi
done

if [ "$REBUILD_NEEDED" = true ]; then
    echo "🔄 Source files changed. Rebuilding..."
    go build -o "$BINARY"
else
    echo "✅ Binary is up to date!"
fi