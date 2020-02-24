#!/bin/sh
set -e
set -x

function mirror_image {
  manifest_dst="${1}"
  shift
  for arg in "$@"; do
    arch=$(cut -d';' -f1 <<<"$arg")
    image_src=$(cut -d';' -f2 <<<"$arg")
    docker pull ${image_src}
    docker tag ${image_src} ${manifest_dst}-${arch}
    docker push ${manifest_dst}-${arch}
    docker manifest create --amend ${manifest_dst} ${manifest_dst}-${arch}
    docker manifest annotate ${manifest_dst} ${manifest_dst}-${arch} --arch ${arch}
  done
  docker manifest push -p ${manifest_dst}
}

# Check to see if a pipe exists on stdin.
if [ -p /dev/stdin ]; then
  echo "Data was piped to this script!"
  # If we want to read the input line by line
  while IFS= read -r line; do
          if [ "$(echo "${line}" | head -c 1)" = "#" ]; then
              continue
          fi
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
            if [ "$(echo "${line}" | head -c 1)" = "#" ]; then
              continue
            fi
            echo "Line: ${line}"
            mirror_image ${line}
          done < "${input}"
  else
          echo "No input given!"
  fi
fi
