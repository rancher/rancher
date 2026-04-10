#!/usr/bin/env bash

set -ex

cd "$(dirname "$0")/.." || return

source scripts/artifacts-list.sh

CHECKSUM_FILE=${CHECKSUM_FILE:-"sha256sum.txt"}
ARTIFACTS_BASE_DIR=${ARTIFACTS_BASE_DIR:-"bin"}

if [[ -z "${ARTIFACTS_TYPE}" ]]; then
  >&2 echo "missing ARTIFACTS_TYPE env var"
  exit 1
fi

if [[ "${ARTIFACTS_TYPE}" != "components" ]] && [[ "${ARTIFACTS_TYPE}" != "digests" ]]; then
  >&2 echo "invalid ARTIFACTS_TYPE, must be either 'components' or 'digests'"
  exit 1
fi

if [[ "${ARTIFACTS_TYPE}" == "digests" ]]; then
  export ARTIFACTS=("${IMAGES_DIGESTS_ARTIFACTS[@]}")
fi

mkdir -p "${ARTIFACTS_BASE_DIR}"
rm "${ARTIFACTS_BASE_DIR}/${CHECKSUM_FILE}" || true
touch "${ARTIFACTS_BASE_DIR}/${CHECKSUM_FILE}"

for artifact in "${ARTIFACTS[@]}"; do
  if [[ ! -f "${ARTIFACTS_BASE_DIR}/${artifact}" ]]; then
    >&2 echo "missing artifact ${ARTIFACTS_BASE_DIR}/${artifact}"
    exit 1
  fi

  sum_file=$(sha256sum "${ARTIFACTS_BASE_DIR}/${artifact}")
  sum=$(echo "$sum_file" | awk '{print $1}')
  echo "$sum $artifact" >>"${ARTIFACTS_BASE_DIR}/${CHECKSUM_FILE}"
done
