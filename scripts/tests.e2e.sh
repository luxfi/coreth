#!/usr/bin/env bash

set -euo pipefail

# Run Lux e2e tests from the target version against the current state of geth.

# e.g.,
# ./scripts/tests.e2e.sh
# LUXD_VERSION=v1.10.x ./scripts/tests.e2e.sh
if ! [[ "$0" =~ scripts/tests.e2e.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Geth root directory
CORETH_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

# Allow configuring the clone path to point to an existing clone
LUXD_CLONE_PATH="${LUXD_CLONE_PATH:-node}"

# Load the version
source "$CORETH_PATH"/scripts/versions.sh

# Always return to the geth path on exit
function cleanup {
  cd "${CORETH_PATH}"
}
trap cleanup EXIT

echo "checking out target Lux version ${LUXD_VERSION}"
if [[ -d "${LUXD_CLONE_PATH}" ]]; then
  echo "updating existing clone"
  cd "${LUXD_CLONE_PATH}"
  git fetch
else
  echo "creating new clone"
  git clone https://github.com/luxfi/node.git "${LUXD_CLONE_PATH}"
  cd "${LUXD_CLONE_PATH}"
fi
# Branch will be reset to $LUXD_VERSION if it already exists
git checkout -B "test-${LUXD_VERSION}" "${LUXD_VERSION}"

echo "updating geth dependency to point to ${CORETH_PATH}"
go mod edit -replace "github.com/luxfi/geth=${CORETH_PATH}"
go mod tidy

echo "building node"
./scripts/build.sh -r

echo "running Lux e2e tests"
E2E_SERIAL=1 ./scripts/tests.e2e.sh --ginkgo.label-filter='c || uses-c'
