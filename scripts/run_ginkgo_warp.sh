#!/usr/bin/env bash

set -euo pipefail

CORETH_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

source "$CORETH_PATH"/scripts/constants.sh

EXTRA_ARGS=()
LUXD_BUILD_PATH="${LUXD_BUILD_PATH:-}"
if [[ -n "${LUXD_BUILD_PATH}" ]]; then
  EXTRA_ARGS=("--luxd-path=${LUXD_BUILD_PATH}/luxd")
  echo "Running with extra args:" "${EXTRA_ARGS[@]}"
fi

"${CORETH_PATH}"/bin/ginkgo -vv --label-filter="${GINKGO_LABEL_FILTER:-}" "${CORETH_PATH}"/tests/warp -- "${EXTRA_ARGS[@]}"
