#!/bin/bash

set -e

BINARY_NAME="tmux-stt"
OUTPUT_DIR="."

echo "Building ${BINARY_NAME}..."

# Build with size optimization
go build -ldflags "-s -w" -o "${OUTPUT_DIR}/${BINARY_NAME}" .

echo "Build complete: ${OUTPUT_DIR}/${BINARY_NAME}"

# Show binary size
SIZE=$(du -h "${OUTPUT_DIR}/${BINARY_NAME}" | cut -f1)
echo "Binary size: ${SIZE}"
