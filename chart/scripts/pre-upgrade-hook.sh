#!/bin/bash

set -eo pipefail

# Global counters
declare -A COUNTS
RESOURCES_FOUND=false

check_prerequisites() {
  if ! command -v kubectl &>/dev/null; then
    echo "Missing required tool: kubectl"
    exit 1
  fi
}

print_resource_table() {
  local kind="$1"
  local items="$2"
  local -a headers=("${@:3}")

  local count
  count=$(wc -l <<< "$items")
  COUNTS["$kind"]=$count
  RESOURCES_FOUND=true

  echo "Found $count $kind resource(s):"
  echo

  IFS=$'\n' read -r -d '' -a lines < <(printf '%s\0' "$items")

  # Initialize max_lengths array with header lengths
  local -a max_lengths
  for i in "${!headers[@]}"; do
    max_lengths[i]=${#headers[i]}
  done

  # Calculate max width for each column
  for line in "${lines[@]}"; do
    IFS=$'\t' read -r -a cols <<< "$line"
    for i in "${!cols[@]}"; do
      (( ${#cols[i]} > max_lengths[i] )) && max_lengths[i]=${#cols[i]}
    done
  done

  for i in "${!headers[@]}"; do
    printf "%-${max_lengths[i]}s  " "${headers[i]}"
  done
  printf "\n"

  for i in "${!headers[@]}"; do
    printf "%-${max_lengths[i]}s  " "$(printf '%*s' "${max_lengths[i]}" '' | tr ' ' '-')"
  done
  printf "\n"

  for line in "${lines[@]}"; do
    IFS=$'\t' read -r -a cols <<< "$line"
    for i in "${!cols[@]}"; do
      printf "%-${max_lengths[i]}s  " "${cols[i]}"
    done
    printf "\n"
  done

  echo
}

detect_resource() {
  local crd="$1"
  local kind="$2"
  local jsonpath="$3"
  local -a headers=("${@:4}")

  echo "Checking for $kind resources..."

  local output
  if ! output=$(kubectl get "$crd" --all-namespaces -o=jsonpath="$jsonpath" 2>&1); then
    if grep -q "the server doesn't have a resource type" <<< "$output"; then
      echo "Resource type $crd not found. Skipping."
      echo
      return 0
    else
      echo "Error retrieving $kind resources: $output"
      exit 1
    fi
  fi

  if [ -z "$output" ]; then
    echo "No $kind resources found."
    echo
  else
    print_resource_table "$kind" "$output" "${headers[@]}"
  fi
}

print_summary() {
  echo "===== SUMMARY ====="
  local total=0
  for kind in "${!COUNTS[@]}"; do
    local count=${COUNTS[$kind]}
    echo "$kind: $count"
    total=$((total + count))
  done

  echo "Total resources detected: $total"

  if [ "$RESOURCES_FOUND" = true ]; then
    echo "Error: Rancher v2.12+ does not support RKE1.
Detected RKE1-related resources (listed above).
Please migrate these clusters to RKE2 or K3s, or delete the related resources.
More info: https://www.suse.com/c/rke-end-of-life-by-july-2025-replatform-to-rke2-or-k3s"
    exit 1
  else
    echo "No RKE related resources found."
  fi
}

main() {
  check_prerequisites

  detect_resource "clusters.management.cattle.io" "RKE Management Cluster" \
    '{range .items[?(@.spec.rancherKubernetesEngineConfig)]}{.metadata.name}{"\t"}{.spec.displayName}{"\n"}{end}' \
    "NAME" "DISPLAY NAME"

  detect_resource "nodetemplates.management.cattle.io" "NodeTemplate" \
    '{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.spec.displayName}{"\n"}{end}' \
    "NAMESPACE" "NAME" "DISPLAY NAME"

  detect_resource "clustertemplates.management.cattle.io" "ClusterTemplate" \
    '{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.spec.displayName}{"\n"}{end}' \
    "NAMESPACE" "NAME" "DISPLAY NAME"

  print_summary
}

main
