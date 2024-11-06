#!/usr/bin/env bash

set -ex

cd "$(dirname "$0")/.." || return

source scripts/artifacts-list.sh

if (( ${#ARTIFACTS[@]} == 0 ));then
  >&2 echo "missing ARTIFACTS env var"
  exit 1
fi

if [[ -z "${ARTIFACTS_BASE_DIR}" ]]; then
  >&2 echo "missing ARTIFACTS_BASE_DIR env var"
  exit 1
fi

CHECKSUM_FILE=${CHECKSUM_FILE:-"dist/sha256sum.txt"}

for artifact in "${ARTIFACTS[@]}"; do
  sum_file=$(sha256sum "$ARTIFACTS_BASE_DIR/$artifact")
  sum=$(echo "$sum_file" | awk '{print $1}')
  echo "$sum $artifact" >> "$CHECKSUM_FILE"
done
