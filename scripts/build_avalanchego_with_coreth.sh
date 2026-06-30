#!/bin/bash

# This script builds a new Luxd binary with the Coreth dependency pointing to the local Coreth path
# Usage: ./build_luxd_with_coreth.sh with optional LUXD_VERSION and LUXD_CLONE_PATH environment variables

set -euo pipefail

# Coreth root directory
CORETH_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

# Allow configuring the clone path to point to an existing clone
LUXD_CLONE_PATH="${LUXD_CLONE_PATH:-luxd}"

# Load the version
source "$CORETH_PATH"/scripts/versions.sh

# Always return to the coreth path on exit
function cleanup {
  cd "${CORETH_PATH}"
}
trap cleanup EXIT

echo "checking out target Luxd version ${LUX_VERSION}"
if [[ -d "${LUXD_CLONE_PATH}" ]]; then
  echo "updating existing clone"
  cd "${LUXD_CLONE_PATH}"
  git fetch
else
  echo "creating new clone"
  git clone https://github.com/luxfi/node.git "${LUXD_CLONE_PATH}"
  cd "${LUXD_CLONE_PATH}"
fi
# Branch will be reset to $LUX_VERSION if it already exists
git checkout -B "test-${LUX_VERSION}" "${LUX_VERSION}"

echo "updating coreth dependency to point to ${CORETH_PATH}"
go mod edit -replace "github.com/luxfi/coreth=${CORETH_PATH}"
go mod tidy

echo "building luxd"
./scripts/build.sh
