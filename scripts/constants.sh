#!/usr/bin/env bash

# Ignore warnings about variables appearing unused since this file is not the consumer of the variables it defines.
# shellcheck disable=SC2034

set -euo pipefail

# Set the PATHS
GOPATH="$(go env GOPATH)"
DEFAULT_PLUGIN_DIR="${HOME}/.node/plugins"
DEFAULT_VM_ID="srEXiWaHuhNyGwPUi444Tu47ZEDwxTWrbQiuD7FmgSAQ6X7Dy"

# Set binary location
binary_path=${GETH_BINARY_PATH:-"$GOPATH/src/github.com/luxfi/node/build/plugins/evm"}

# Lux docker hub
DOCKERHUB_REPO="luxfi/geth"

# Current branch
CURRENT_BRANCH=${CURRENT_BRANCH:-$(git describe --tags --exact-match 2>/dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD)}
echo "Using branch: ${CURRENT_BRANCH}"

# Image build id
# Use an abbreviated version of the full commit to tag the image.

# WARNING: this will use the most recent commit even if there are un-committed changes present
GETH_COMMIT="$(git --git-dir="$GETH_PATH/.git" rev-parse HEAD)"

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"
