#!/usr/bin/env bash

set -ex

cd "$(dirname "$0")/.." || return

CHECKSUM_FILE=${CHECKSUM_FILE:-"sha256sum.txt"}

if [[ -z "${ARTIFACTS_TYPE}" ]]; then
  >&2 echo "missing ARTIFACTS_TYPE env var"
  exit 1
fi

case $ARTIFACTS_TYPE in
  components)
    source scripts/artifacts-list.sh
    ;;
  digests)
    export ARTIFACTS=(
      "$(basename "$LINUX_AMD64_FILE")"
      "$(basename "$LINUX_ARM64_FILE")"
      "$(basename "$WINDOWS_2019_FILE")"
      "$(basename "$WINDOWS_2022_FILE")"
    )
    ;;
  *)
    >&2 echo "invalid ARTIFACTS_TYPE"
    exit 1
esac


if [[ -z "${ARTIFACTS_BASE_DIR}" ]]; then
  >&2 echo "missing ARTIFACTS_BASE_DIR env var"
  exit 1
fi

rm "$ARTIFACTS_BASE_DIR/$CHECKSUM_FILE" || true
touch "$ARTIFACTS_BASE_DIR/$CHECKSUM_FILE"

for artifact in "${ARTIFACTS[@]}"; do
  if [[ -z "$artifact" ]]; then
    >&2 echo "missing artifact"
    exit 1
  fi

  sum_file=$(sha256sum "$ARTIFACTS_BASE_DIR/$artifact")
  sum=$(echo "$sum_file" | awk '{print $1}')
  echo "$sum $artifact" >> "$ARTIFACTS_BASE_DIR/$CHECKSUM_FILE"
done
