#!/bin/bash

set -ex

# destination file for the checksums
if [[ -z "$CHECKSUM_FILE" ]]; then
  >&2 echo "missing CHECKSUM_FILE env var"
  exit 1
fi

if [[ -z "$LINUX_AMD64_FILE" ]]; then
  >&2 echo "missing LINUX_AMD64_FILE env var"
  exit 1
fi

if [[ -z "$LINUX_ARM64_FILE" ]]; then
  >&2 echo "missing LINUX_ARM64_FILE env var"
  exit 1
fi

if [[ -z "$WINDOWS_2019_FILE" ]]; then
  >&2 echo "missing WINDOWS_2019_FILE env var"
  exit 1
fi

if [[ -z "$WINDOWS_2022_FILE" ]]; then
  >&2 echo "missing WINDOWS_2022_FILE env var"
  exit 1
fi

append_sum() {
  file=$1
  if [[ -z "$file" ]]; then
    >&2 echo "missing file to generate sum"
    exit 1
  fi

  sum_file=$(sha256sum "$file")
  sum=$(echo "$sum_file" | awk '{print $1}')
  file_name=$(basename "$file")
  echo "$file_name $sum" >> "$CHECKSUM_FILE"
}

rm "$CHECKSUM_FILE" && touch "$CHECKSUM_FILE"

append_sum "$LINUX_AMD64_FILE"
append_sum "$LINUX_ARM64_FILE"
append_sum "$WINDOWS_2019_FILE"
append_sum "$WINDOWS_2022_FILE"
