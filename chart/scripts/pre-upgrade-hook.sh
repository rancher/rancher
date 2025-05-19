#!/bin/bash

set -eo pipefail

# Global counters
declare -A COUNTS
RESOURCES_FOUND=false

check_prerequisites() {
  local missing_tools=()

  for tool in kubectl jq; do
    command -v "$tool" &>/dev/null || missing_tools+=("$tool")
  done

  if [ ${#missing_tools[@]} -gt 0 ]; then
    echo "Missing required tools: ${missing_tools[*]}"
    exit 1
  fi
}

print_resource_table() {
  local kind="$1"
  local items="$2"

  local count
  count=$(wc -l <<< "$items")
  COUNTS["$kind"]=$count
  RESOURCES_FOUND=true

  echo "Found $count $kind resource(s):"

  IFS=$'\n' read -r -d '' -a lines < <(printf '%s\0' "$items")

  local max_ns_len=9
  local max_name_len=4
  local max_display_len=12

  for line in "${lines[@]}"; do
    IFS=$'\t' read -r ns name display <<< "$line"
    (( ${#ns} > max_ns_len )) && max_ns_len=${#ns}
    (( ${#name} > max_name_len )) && max_name_len=${#name}
    (( ${#display} > max_display_len )) && max_display_len=${#display}
  done

  printf "%-${max_ns_len}s  %-${max_name_len}s  %-${max_display_len}s\n" "NAMESPACE" "NAME" "DISPLAY NAME"
  printf "%-${max_ns_len}s  %-${max_name_len}s  %-${max_display_len}s\n" \
    "$(printf '%*s' "$max_ns_len" '' | tr ' ' '-')" \
    "$(printf '%*s' "$max_name_len" '' | tr ' ' '-')" \
    "$(printf '%*s' "$max_display_len" '' | tr ' ' '-')"

  for line in "${lines[@]}"; do
    IFS=$'\t' read -r ns name display <<< "$line"
    printf "%-${max_ns_len}s  %-${max_name_len}s  %-${max_display_len}s\n" "$ns" "$name" "$display"
  done

  echo
}

detect_resource() {
  local crd="$1"
  local kind="$2"
  local filter="$3"

  echo "Checking for $kind resources..."

  local json
  if ! json=$(kubectl get "$crd" --all-namespaces -o json 2>&1); then
    if grep -q "the server doesn't have a resource type" <<< "$json"; then
      echo "Resource type $crd not found. Skipping."
      echo
      return 0
    else
      echo "Error retrieving $kind resources: $json"
      exit 1
    fi
  fi

  local items
  items=$(jq -r "$filter" <<< "$json")

  if [ -z "$items" ]; then
    echo "No $kind resources found."
    echo
  else
    print_resource_table "$kind" "$items"
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
    echo "Error: RKE related resources were found, please delete the mentioned resources to continue"
    exit 1
  else
    echo "No RKE related resources found."
  fi
}

main() {
  check_prerequisites

  detect_resource "clusters.management.cattle.io" "RKE Management Cluster" \
    '.items[] | select(.spec.rancherKubernetesEngineConfig != null) | 
     [.metadata.namespace // "default", .metadata.name, .spec.displayName // ""] | @tsv'

  detect_resource "nodetemplates.management.cattle.io" "NodeTemplate" \
    '.items[] | [.metadata.namespace // "default", .metadata.name, .spec.displayName // ""] | @tsv'

  detect_resource "clustertemplates.management.cattle.io" "ClusterTemplate" \
    '.items[] | [.metadata.namespace // "default", .metadata.name, .spec.displayName // ""] | @tsv'

  print_summary
}

main
