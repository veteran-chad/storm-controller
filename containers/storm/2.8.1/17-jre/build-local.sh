#!/bin/bash

# Simple wrapper for local development builds

VERSION="${1:-2.8.1}"
TAG="${2:-17-jre}"

echo "Building local Storm image: storm-local (version=$VERSION, tag=$TAG)"
echo

docker build \
    --build-arg VERSION="$VERSION" \
    --build-arg TAG="$TAG" \
    --build-arg SERVICE=storm \
    --build-arg ENVIRONMENT=local \
    --tag storm-local \
    .

if [ $? -eq 0 ]; then
    echo
    echo "Local build successful!"
    echo "Image tagged as: storm-local"
    echo
    echo "To run:"
    echo "  docker run --rm storm-local storm version"
else
    echo "Build failed!"
    exit 1
fi