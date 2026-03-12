#!/bin/bash
# Travis CI image build script for Blog
# Builds multi-arch Docker image and pushes to Docker Hub
#
# Required Travis CI env vars:
#   DOCKERHUB_USERNAME, DOCKERHUB_TOKEN

set -euo pipefail

IMAGE_NAME="anchoo2kewl/blog"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-$(git rev-parse --short HEAD)")
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')

echo "=== Building Blog Image ==="
echo "Image:   $IMAGE_NAME"
echo "Version: $VERSION"
echo "Commit:  $GIT_COMMIT"
echo ""

# Login to Docker Hub
echo "$DOCKERHUB_TOKEN" | docker login -u "$DOCKERHUB_USERNAME" --password-stdin

# Set up buildx for multi-arch
docker buildx create --name multiarch --use 2>/dev/null || docker buildx use multiarch

# Build and push multi-arch image
docker buildx build \
  --platform linux/arm64,linux/amd64 \
  --file Dockerfile \
  --build-arg "VERSION=$VERSION" \
  --build-arg "GIT_COMMIT=$GIT_COMMIT" \
  --build-arg "BUILD_TIME=$BUILD_TIME" \
  --build-arg "GO_VERSION=$GO_VERSION" \
  --tag "$IMAGE_NAME:latest" \
  --tag "$IMAGE_NAME:$GIT_COMMIT" \
  --push \
  .

echo ""
echo "=== Blog image pushed ==="
echo "Tags: $IMAGE_NAME:latest, $IMAGE_NAME:$GIT_COMMIT"
