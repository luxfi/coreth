#!/usr/bin/env bash

set -euo pipefail

# Lux root directory
CORETH_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

# Load the constants
source "$CORETH_PATH"/scripts/constants.sh

# Load the versions
source "$CORETH_PATH"/scripts/versions.sh

# WARNING: this will use the most recent commit even if there are un-committed changes present
BUILD_IMAGE_ID=${BUILD_IMAGE_ID:-"${CURRENT_BRANCH}"}
echo "Building Docker Image: $DOCKERHUB_REPO:$BUILD_IMAGE_ID based of Lux Node@$LUX_VERSION"
docker build -t "$DOCKERHUB_REPO:$BUILD_IMAGE_ID" "$CORETH_PATH" -f "$CORETH_PATH/Dockerfile" \
  --build-arg LUX_VERSION="$LUX_VERSION" \
  --build-arg CORETH_COMMIT="$CORETH_COMMIT" \
  --build-arg CURRENT_BRANCH="$CURRENT_BRANCH"
