#!/bin/bash

# Helper script to run Tastora tests with proper Docker environment
# This script sets the correct DOCKER_HOST for Docker Desktop on macOS

echo "Setting up Docker environment for Tastora tests..."

# Check if Docker is running
if ! docker ps > /dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker Desktop and try again."
    exit 1
fi

# Set the correct Docker host for Docker Desktop on macOS
export DOCKER_HOST=unix:///Users/$(whoami)/.docker/run/docker.sock

echo "Docker environment configured:"
echo "DOCKER_HOST=$DOCKER_HOST"

# Check if we can connect to Docker
if ! docker ps > /dev/null 2>&1; then
    echo "Warning: Cannot connect to Docker with the configured socket."
    echo "Falling back to default Docker socket..."
    unset DOCKER_HOST
fi

echo "Running Tastora tests..."

# Change to project root directory
cd "$(dirname "$0")/../../.."

# Run the tests
if [ "$1" == "blob" ]; then
    echo "Running blob tests only..."
    make test-blob
elif [ "$1" == "all" ]; then
    echo "Running all tastora tests..."
    make test-tastora
else
    echo "Running all tastora tests (default)..."
    make test-tastora
fi 