#!/bin/bash

# Development wrapper that auto-rebuilds if needed before running

cd "$(dirname "$0")/.." || exit 1

# Run smart build
./scripts/smart-build.sh > /dev/null 2>&1

# Run the command
./ipcrawler "$@"