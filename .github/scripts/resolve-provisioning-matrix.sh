#!/usr/bin/env bash
# =============================================================================
#
# This script generates the appropriate GitHub Actions provisioning tests matrix
# (as JSON) for the changed files in the current working tree. It is used by
# the provisioning workflow to conditionally run only the tests that are
# relevant to the changes in a PR.
#
# It works in three steps:
#   Step 1: if EXPLICIT_SCOPES is passed, immediately resolve those scopes. An explicit scope
#           is never triggered by a changed file, so change matching is skipped entirely.
#           Each explicit scope is tracked in a separate '.explicit' array in
#           provisioning-test-scopes.yaml,
#   Step 2 (otherwise): get the list of changed files (or "run everything" when ALL=1), then
#           pick the "scopes" whose path patterns match a changed file.
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
EXPLICIT_SCOPE_NAMES="$(yq '.explicit // [] | map(.name) | join(" ")' "$CONFIG")"

# The ref to diff against. CI passes the target branch's SHA
# via DIFF_BASE. Locally it falls back to the branch's upstream tracking ref.
if [ -z "${DIFF_BASE:-}" ]; then
  DIFF_BASE='@{upstream}'
fi

# Stop early if a prerequisite is missing. Shouldn't happen, rancher managed CI images
# include jq and yq as universal deps.
test -f "$CONFIG"        || { echo "error: config not found: $CONFIG" >&2; exit 1; }
command -v yq >/dev/null || { echo "error: yq (v4) is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

# ALL=1 means "run the whole matrix", otherwise fall back to computing it from git.
# CHANGED can also be used, and is expected to be a newline-separated list of file paths.
ALL="${ALL:-0}"

# EXPLICIT_SCOPES opts into one or more explicit-only scopes by name (space-separated, e.g.
# "nightly"). It is intentionally kept separate from ALL/full so those scopes never get pulled
# into a normal PR's matrix. When set, it short-circuits everything else below -- there's no
# need to diff or match against changed files at all.
EXPLICIT_SCOPES="${EXPLICIT_SCOPES:-}"

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

  # These files already had their chance to trigger the scope above and didn't, so they're
  # removed from the pool before checking package-paths.
  local claimed remaining
  claimed="$(files_matching_file_paths ".scopes.$scope.file-paths" "$CHANGED")"
  remaining="$(exclude_files "$CHANGED" "$claimed")"

  list_matches ".scopes.$scope.package-paths" "$scope" "$remaining"
}

# files_matching_file_paths prints every changed file that matches the path regex of any entry in
# the given file-paths list, regardless of whether its substring check passed.
files_matching_file_paths() {
  local yq_path="$1"
  local files_pool="$2"
  local n i path

  n="$(yq "($yq_path // []) | length" "$CONFIG")"
  for ((i = 0; i < n; i++)); do
    path="$(yq "$yq_path[$i] | (.path // .)" "$CONFIG")"
    test -n "$path" || continue
    printf '%s\n' "$files_pool" | grep -E "$path" || true
  done
}

# exclude_files prints every line of files_pool that isn't also a line in exclude_list.
exclude_files() {
  local files_pool="$1"
  local exclude_list="$2"

  test -n "$exclude_list" || { printf '%s\n' "$files_pool"; return; }

  printf '%s\n' "$files_pool" | grep -vFxf <(printf '%s\n' "$exclude_list") || true
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
  changed_lines="$(git diff -U0 "$DIFF_BASE...HEAD" -- "$file" | grep -E '^[+-]' | grep -Ev '^(\+\+\+|---)')"
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

# add_scope appends $2 to the space-separated scope list in $1 and prints the result.
add_scope() {
  local list="$1" scope="$2"
  if [ -z "$list" ]; then
    printf '%s' "$scope"
  else
    printf '%s %s' "$list" "$scope"
  fi
}

# resolve_explicit_scopes prints the subset of EXPLICIT_SCOPES that are actually declared
# under the config's 'explicit' array.
resolve_explicit_scopes() {
  local selected="" scope
  for scope in $EXPLICIT_SCOPES; do
    if ! printf '%s\n' "$EXPLICIT_SCOPE_NAMES" | grep -qw -- "$scope"; then
      echo "fatal: requested scope '$scope', not declared under 'explicit' in $CONFIG" >&2
      return 1
    fi
    selected="$(add_scope "$selected" "$scope")"
  done
  if [ -z "$selected" ]; then
    echo "fatal: EXPLICIT_SCOPES was set but resolved to no scopes" >&2
    return 1
  fi
  printf '%s' "$selected"
}

# resolve_changed_scopes prints every scope in $SCOPES matching CHANGED (or every scope, if
# ALL=1 or a 'full' path matched). It also fills in the global CHANGED if it wasn't passed in
# by looking at the current git diff.
resolve_changed_scopes() {
  CHANGED="${CHANGED:-}"
  if [ "$ALL" != "1" ] && [ -z "$CHANGED" ]; then
    CHANGED="$(git diff --name-only "$DIFF_BASE...HEAD")"
  fi

  if [ "$ALL" = "1" ] || list_matches '.full' "full" "$CHANGED"; then
    printf '%s' "$SCOPES"
    return
  fi

  local selected="" scope
  for scope in $SCOPES; do
    if scope_matches "$scope"; then
      selected="$(add_scope "$selected" "$scope")"
    fi
  done
  printf '%s' "$selected"
}

# Explicit scopes bypass CHANGED/ALL/full entirely -- they're never matched by path, so
# there's nothing to diff or check changed files against when they're requested.
if [ -n "$EXPLICIT_SCOPES" ]; then
  selected="$(resolve_explicit_scopes)" || exit 1
else
  selected="$(resolve_changed_scopes)" || exit 1
fi

echo "resolved selected scopes: [$selected]" >&2

# Convert the YAML to JSON, then let jq collect the test rows for the picked
# scopes and rename the friendly keys to the ones the workflow reads:
#   dist -> V2PROV_TEST_DIST | regex -> V2PROV_TEST_RUN_REGEX | features -> CATTLE_FEATURES
# Each test lives in exactly one scope (see the YAML invariants), so no de-duplication
# is needed. An empty `selected` will return {"include":[]}. This block also controls the
# default configuration of the underlying ec2 runners, unless overridden by a specific test.
#
# A scope's tests are normally read from '.scopes.<name>.tests'; explicit-only scopes are
# instead entries of the top-level '.explicit' array, keyed by their 'name' field.
yq -o=json '.' "$CONFIG" | jq -c --arg selected "$selected" '
  (.explicit // [] | map({(.name): .}) | add // {}) as $explicit
  | {
    include: [
      ($selected | split(" ") | map(select(length > 0))[]) as $scope
      | ((.scopes[$scope] // $explicit[$scope] // {}).tests // [])[]
      | { V2PROV_TEST_DIST: .dist, V2PROV_TEST_RUN_REGEX: .regex }
        + (if .features then { CATTLE_FEATURES: .features } else {} end)
        + (if .cpus then { V2PROV_TEST_CPUS: .cpus } else { V2PROV_TEST_CPUS: 16 } end)
        + (if .parallelism then { V2PROV_TEST_PARALLELISM: .parallelism } else { V2PROV_TEST_PARALLELISM: 1 } end)
    ]
  }
'
