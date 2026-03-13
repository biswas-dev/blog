#!/bin/bash
# Travis CI image build script for Blog
# Builds multi-arch Docker image and pushes to Harbor registry
# Tags with git-<sha> (immutable) and staging-latest
# Captures and persists the image digest for deploy-staging to consume
#
# Required Travis CI env vars:
#   HARBOR_AUTH (base64-encoded "username:password" — avoids $ shell expansion)

set -euo pipefail

IMAGE_NAME="harbor.biswas.me/biswas/blog"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-$(git rev-parse --short HEAD)")
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')
TAG="git-${GIT_COMMIT}"

# Decode Harbor credentials (base64 avoids Travis $ expansion issues)
HARBOR_USERNAME=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f1)
HARBOR_PASSWORD=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f2)

echo "=== Building Blog Image ==="
echo "Image:   $IMAGE_NAME"
echo "Tag:     $TAG"
echo "Version: $VERSION"
echo "Commit:  $GIT_COMMIT"
echo ""

# Login to Harbor
echo "$HARBOR_PASSWORD" | docker login harbor.biswas.me -u "$HARBOR_USERNAME" --password-stdin

# Set up buildx for multi-arch
docker buildx create --name multiarch --use 2>/dev/null || docker buildx use multiarch

# Build and push multi-arch image with metadata capture
docker buildx build \
  --platform linux/arm64,linux/amd64 \
  --file Dockerfile \
  --build-arg "VERSION=$VERSION" \
  --build-arg "GIT_COMMIT=$GIT_COMMIT" \
  --build-arg "BUILD_TIME=$BUILD_TIME" \
  --build-arg "GO_VERSION=$GO_VERSION" \
  --tag "$IMAGE_NAME:$TAG" \
  --metadata-file /tmp/build-metadata.json \
  --push \
  .

# Extract digest from build metadata
DIGEST=$(jq -r '.["containerimage.digest"]' /tmp/build-metadata.json)

if [ -z "$DIGEST" ] || [ "$DIGEST" = "null" ]; then
  echo "ERROR: Failed to extract digest from build metadata"
  cat /tmp/build-metadata.json
  exit 1
fi

echo ""
echo "Digest: $DIGEST"

# Tag as staging-latest via Harbor API
curl -sf -u "$HARBOR_USERNAME:$HARBOR_PASSWORD" \
  -X POST "https://harbor.biswas.me/api/v2.0/projects/biswas/repositories/blog/artifacts/$DIGEST/tags" \
  -H "Content-Type: application/json" \
  -d '{"name": "staging-latest"}'

echo "Tagged as staging-latest"

# Persist digest for the deploy-staging stage (Travis workspace shares this)
echo "$DIGEST" > .image-digest.txt

echo ""
echo "=== Blog image pushed ==="
echo "Image: $IMAGE_NAME:$TAG"
echo "Digest: $DIGEST"
