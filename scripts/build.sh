#!/bin/bash

# Build script for ipcrawler
# This script builds the binary in place, allowing the symlink to always point to the latest version

echo "Building ipcrawler..."
go build -o ipcrawler

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo "The ipcrawler command is available globally via symlink at: $(which ipcrawler)"
else
    echo "Build failed!"
    exit 1
fi