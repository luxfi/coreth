#!/usr/bin/env bash

set -euo pipefail

# Run LuxGo e2e tests from the target version against the current state of coreth.

# e.g.,
# ./scripts/tests.e2e.sh
# LUX_VERSION=v1.10.x ./scripts/tests.e2e.sh
if ! [[ "$0" =~ scripts/tests.e2e.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Coreth root directory
CORETH_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Allow configuring the clone path to point to an existing clone
LUXGO_CLONE_PATH="${LUXGO_CLONE_PATH:-node}"

# Load the version
source "$CORETH_PATH"/scripts/versions.sh

# Always return to the coreth path on exit
function cleanup {
  cd "${CORETH_PATH}"
}
trap cleanup EXIT

echo "checking out target LuxGo version ${avalanche_version}"
if [[ -d "${LUXGO_CLONE_PATH}" ]]; then
  echo "updating existing clone"
  cd "${LUXGO_CLONE_PATH}"
  git fetch
else
  echo "creating new clone"
  git clone https://github.com/luxdefi/node.git "${LUXGO_CLONE_PATH}"
  cd "${LUXGO_CLONE_PATH}"
fi
# Branch will be reset to $avalanche_version if it already exists
git checkout -B "test-${avalanche_version}" "${avalanche_version}"

echo "updating coreth dependency to point to ${CORETH_PATH}"
go mod edit -replace "github.com/luxdefi/coreth=${CORETH_PATH}"
go mod tidy

echo "building node"
./scripts/build.sh -r

echo "running LuxGo e2e tests"
E2E_SERIAL=1 ./scripts/tests.e2e.sh --ginkgo.label-filter='c || uses-c'
