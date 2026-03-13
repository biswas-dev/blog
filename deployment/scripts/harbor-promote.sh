#!/bin/bash
# Promotes an image digest from one environment to the next via Harbor API.
# Usage: harbor-promote.sh <project> <repo> <source-env> <target-env>
# Example: harbor-promote.sh biswas blog staging uat
#
# Finds the digest tagged as <source-env>-latest, adds <target-env>-latest tag.
# Outputs the digest for use in deployment.
#
# Required env vars: HARBOR_USERNAME, HARBOR_PASSWORD (or HARBOR_AUTH base64)

set -euo pipefail

# Decode Harbor credentials if using base64 format
if [ -n "${HARBOR_AUTH:-}" ]; then
  HARBOR_USERNAME=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f1)
  HARBOR_PASSWORD=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f2)
fi

PROJECT="${1:?Usage: harbor-promote.sh <project> <repo> <source-env> <target-env>}"
REPO="$2"
SOURCE_ENV="$3"
TARGET_ENV="$4"

HARBOR_URL="https://harbor.biswas.me"

echo "=== Promoting $REPO: $SOURCE_ENV -> $TARGET_ENV ===" >&2

# Get digest of source environment's latest
DIGEST=$(curl -sf -u "$HARBOR_USERNAME:$HARBOR_PASSWORD" \
  "$HARBOR_URL/api/v2.0/projects/$PROJECT/repositories/$REPO/artifacts?q=tags%3D${SOURCE_ENV}-latest" \
  | jq -r '.[0].digest')

if [ -z "$DIGEST" ] || [ "$DIGEST" = "null" ]; then
  echo "ERROR: No artifact found with tag ${SOURCE_ENV}-latest in $PROJECT/$REPO" >&2
  exit 1
fi

echo "Found digest: $DIGEST" >&2

# Tag for target environment
curl -sf -u "$HARBOR_USERNAME:$HARBOR_PASSWORD" \
  -X POST "$HARBOR_URL/api/v2.0/projects/$PROJECT/repositories/$REPO/artifacts/$DIGEST/tags" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"${TARGET_ENV}-latest\"}"

echo "Tagged as ${TARGET_ENV}-latest" >&2

# Output only the digest (for capture by caller)
echo "$DIGEST"
