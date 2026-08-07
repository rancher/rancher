#!/usr/bin/env bash
# =============================================================================
#
# This script generates the appropriate GitHub Actions provisioning tests matrix
# (as JSON) for the changed files in the current working tree. It is used by
# the provisioning workflow to conditionally run only the tests that are
# relevant to the changes in a PR.
#
# It works in three steps:
#   Step 1: get the list of changed files (or "run everything" when ALL=1).
#   Step 2: pick the "scopes" whose path patterns match a changed file.
#     2a: If a file is explicitly mentioned in the scopes 'file-paths' list,
#          that scope is picked if the file's changed lines contain one
#          of the listed substrings (or unconditionally if no substrings are listed).
#     2b: If a file is not explicitly mentioned in the scopes 'file-paths' list,
#          it is matched against the scopes 'package-paths' list.
#   Step 3: print the picked scopes' test rows as {"include":[ ... ]}.
#
# The rules which map paths to specific scopes, and denote which tests each scope runs,
# live entirely in provisioning-test-scopes.yaml.
#
# Output:
#   {"include":[{"V2PROV_TEST_DIST":"k3s","V2PROV_TEST_RUN_REGEX":"^Test_...$"}, ...]}
#   Nothing picked -> {"include":[]}
#
# =============================================================================
set -uo pipefail

# The rules file sits next to this script (SCOPES_CONFIG can override it for tests)
DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG="${SCOPES_CONFIG:-$DIR/provisioning-test-scopes.yaml}"
SCOPES="$(yq '.scopes | keys | join(" ")' "$CONFIG")"

UPSTREAM_REF="@{upstream}"

# Stop early if a prerequisite is missing. Shouldn't happen, rancher managed CI images
# include jq and yq as universal deps.
test -f "$CONFIG"        || { echo "error: config not found: $CONFIG" >&2; exit 1; }
command -v yq >/dev/null || { echo "error: yq (v4) is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

# ALL=1 means "run the whole matrix", otherwise fall back to computing it from git.
# CHANGED can also be used, and is expected to be a newline-separated list of file paths.
ALL="${ALL:-0}"
if [ "$ALL" != "1" ] && [ -z "${CHANGED:-}" ]; then
  CHANGED="$(git diff --name-only "$UPSTREAM_REF")"
fi
CHANGED="${CHANGED:-}"

# scope_matches checks if the changed files trigger any of the defined scopes. This is the case if
# a changed file is either:
#   1. explicitly mentioned in the scopes 'file-paths' list, and any of the changed lines in
#     that file match one of the listed substrings (or unconditionally, if no substrings are listed)
#   2. matches one of the scopes 'package-paths' entries.
scope_matches() {
  local scope="$1"

  if list_matches ".scopes.$scope.file-paths" "$scope" "$CHANGED"; then
    return 0
  fi

  list_matches ".scopes.$scope.package-paths" "$scope" "$CHANGED"
}

# list_matches iterates over the list of file-paths or package-paths for a given scope,
# and checks if any of the changed files match.
list_matches() {
  local yq_path="$1"
  local label="$2"
  local files_pool="$3"
  local n

  n="$(yq "($yq_path // []) | length" "$CONFIG")"
  local i

  # iterate over all of the paths for the given scope
  # (indicated by label and yq_path, which denote the
  # scope and the list of paths, respectively)
  for ((i = 0; i < n; i++)); do
    local path files

    # for each entry collect the actual file path
    # and all of the associated substrings
    path="$(yq "$yq_path[$i] | (.path // .)" "$CONFIG")"
    test -n "$path" || continue

    # Convert the list of changed files into a newline delimited list,
    # and filter it down to only the files that match the current path regex.
    # If none of the changed files match the current path regex,
    # continue to the next path in the list.
    files="$(printf '%s\n' "$files_pool" | grep -E "$path")" || continue

    # collect all of the substring rules for the current path entry
    local substrings
    substrings="$(yq "$yq_path[$i] | (.substrings // [])[]" "$CONFIG")"

    # This handles the case where a scope is triggered by a package-path, as well as the case
    # where a scope is triggered by a file-path with no substring rules.
    if [ -z "$substrings" ]; then
      printf "%s matches provisioning test scope '%s' via path '%s'\n" "$files" "$label" "$path" >&2
      return 0
    fi

    # Iterate over all of the changed files which match the current path regex,
    # and check if any of them have a changed line that matches one of the substring rules.
    local file
    while IFS= read -r file; do
      test -n "$file" || continue
      if file_has_substring "$file" "$substrings"; then
        printf "%s matches provisioning test scope '%s' via path '%s' substring\n" "$file" "$label" "$path" >&2
        return 0
      fi
    done <<< "$files"
  done
  return 1
}

# file_has_substring checks if any _changed_ line (added or removed, from a zero-context diff)
# in $file contains one of the literal $substrings (newline-separated).
file_has_substring() {
  local file="$1"
  local substrings="$2"

  # only the actual added/removed lines, not the "+++ / ---" file headers or unchanged context.
  local changed_lines
  changed_lines="$(git diff -U0 "$UPSTREAM_REF" -- "$file" 2>/dev/null | grep -E '^[+-]' | grep -Ev '^(\+\+\+|---)')"
  test -n "$changed_lines" || return 1

  # check each substring one at a time against the changed lines, the first hit wins.
  local s
  while IFS= read -r s; do
    test -n "$s" || continue
    if printf '%s\n' "$changed_lines" | grep -qF -- "$s"; then
      printf "%s has changed lines matching substring '%s'\n" "$file" "$s" >&2
      return 0
    fi
  done <<< "$substrings"
  return 1
}

selected=""
if [ "$ALL" = "1" ] || list_matches '.full' "full" "$CHANGED"; then
  selected="$SCOPES"
else
  for scope in $SCOPES; do
    if scope_matches "$scope"; then
      if [ -z "$selected" ]; then
        selected="$scope"
      else
        selected="$selected $scope"
      fi
    fi
  done
fi

echo "resolved selected scopes: [$selected]" >&2

# Convert the YAML to JSON, then let jq collect the test rows for the picked
# scopes and rename the friendly keys to the ones the workflow reads:
#   dist -> V2PROV_TEST_DIST | regex -> V2PROV_TEST_RUN_REGEX | features -> CATTLE_FEATURES
# Each test lives in exactly one scope (see the YAML invariants), so no de-duplication
# is needed. An empty `selected` will return {"include":[]}.
yq -o=json '.' "$CONFIG" | jq -c --arg selected "$selected" '
  {
    include: [
      ($selected | split(" ") | map(select(length > 0))[]) as $scope
      | (.scopes[$scope].tests // [])[]
      | { V2PROV_TEST_DIST: .dist, V2PROV_TEST_RUN_REGEX: .regex }
        + (if .features then { CATTLE_FEATURES: .features } else {} end)
        + (if .cpus then { V2PROV_TEST_CPUS: .cpus } else { V2PROV_TEST_CPUS: 16 } end)
    ]
  }
'
