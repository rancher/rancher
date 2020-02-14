#!/bin/sh
set -e

function mirror_image {
  image_src="${1}"
  image_dst="${2}"
  image_tag="${3}"

  docker pull "${image_src}:${image_tag}"
  docker tag "${image_src}:${image_tag}" "${image_dst}:${image_tag}"
  docker push "${image_dst}:${image_tag}"
}

# Check to see if a pipe exists on stdin.
if [ -p /dev/stdin ]; then
  echo "Data was piped to this script!"
  # If we want to read the input line by line
  while IFS= read -r line; do
          echo "Line: ${line}"
          mirror_image ${line}
  done
else
  echo "No input was found on stdin, skipping!"
  # Checking to ensure a filename was specified and that it exists
  if [ -f "$1" ]; then
          echo "Filename specified: ${1}"
          input="${1}"
          while IFS= read -r line
          do
            echo "Line: ${line}"
            mirror_image ${line}
          done < "${input}"
  else
          echo "No input given!"
  fi
fi
