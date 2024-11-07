#!/bin/bash

# this list doesn't include sha256sum.txt and the rancher-images-digests-*.txt files
export ARTIFACTS=(
  "rancher-components.txt"
  "rancher-data.json"
  "rancher-images-origins.txt"
  "rancher-images-sources.txt"
  "rancher-images.txt"
  "rancher-load-images.ps1"
  "rancher-load-images.sh"
  "rancher-mirror-to-rancher-org.ps1"
  "rancher-mirror-to-rancher-org.sh"
  "rancher-rke-k8s-versions.txt"
  "rancher-save-images.ps1"
  "rancher-save-images.sh"
  "rancher-windows-images-sources.txt"
  "rancher-windows-images.txt"
)
