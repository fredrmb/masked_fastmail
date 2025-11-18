#!/bin/bash
# Build script for masked_fastmail with version information

set -e

# Get version information from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build with ldflags
go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o masked_fastmail

echo "Built masked_fastmail with version: ${VERSION}"

