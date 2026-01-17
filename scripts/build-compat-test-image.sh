#!/bin/bash

set -e

VERSION="${1:-}"
PUSH="${2:-}"

if [ -z "$VERSION" ]; then
    echo "Error: Version is required"
    echo "Usage: $0 <version> [push]"
    echo "Example: $0 v0.28.3-arabica"
    echo "Example: $0 v0.28.3-arabica push"
    exit 1
fi

IMAGE_NAME="ghcr.io/celestiaorg/celestia-node-compat-test:${VERSION}"

echo "Building compat-test image for version: $VERSION"
echo "Image name: $IMAGE_NAME"

CURRENT_VERSION=$(git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD)
echo "Current git version: $CURRENT_VERSION"

if git rev-parse "$VERSION" >/dev/null 2>&1; then
    if [ "$(git rev-parse HEAD)" != "$(git rev-parse "$VERSION")" ]; then
        echo "Checking out version $VERSION..."
        git checkout "$VERSION"
        trap "git checkout -" EXIT
    fi
else
    echo "Warning: Version $VERSION not found in git. Building from current commit."
fi

echo "Building Docker image..."
docker build -f cmd/compat-test/Dockerfile -t "$IMAGE_NAME" .

if [ "$PUSH" == "push" ]; then
    echo "Pushing image to registry..."
    docker push "$IMAGE_NAME"
    echo "Image pushed successfully: $IMAGE_NAME"
else
    echo "Image built successfully: $IMAGE_NAME"
    echo "To push the image, run: docker push $IMAGE_NAME"
    echo "Or run this script with 'push' as second argument"
fi
