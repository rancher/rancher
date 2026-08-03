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
#   Step 3: print the picked scopes' test rows as {"include":[ ... ]}.
#
# The rules (which map paths to specific scopes, and denote which tests each scope runs)
# live entirely in provisioning-test-scopes.yaml.
#
# Output:
#   {"include":[{"V2PROV_TEST_DIST":"k3s","V2PROV_TEST_RUN_REGEX":"^Test_...$"}, ...]}
#   Nothing picked -> {"include":[]}
#
# =============================================================================
set -uo pipefail

# The rules file sits next to this script (SCOPES_CONFIG can override it for tests).
DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG="${SCOPES_CONFIG:-$DIR/provisioning-test-scopes.yaml}"
SCOPES="$(yq '.scopes | keys | join(" ")' "$CONFIG")"

# Stop early if a prerequisite is missing. Shouldn't happen, rancher managed CI images
# include jq and yq as universal deps.
test -f "$CONFIG"        || { echo "error: config not found: $CONFIG" >&2; exit 1; }
command -v yq >/dev/null || { echo "error: yq (v4) is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }

# ALL=1 means "run the whole matrix", otherwise fall back to computing it from git.
# CHANGED can also be used, and is expected to be a newline-separated list of file paths.
ALL="${ALL:-0}"
if [ "$ALL" != "1" ] && [ -z "${CHANGED:-}" ]; then
  BASE="${1:-origin/main}"
  CHANGED="$(git diff --name-only @{upstream})"
fi
CHANGED="${CHANGED:-}"

matches() {
  local pattern="$1"
  local scope="$2"
  test -n "$pattern" || return 1
  printf '%s\n' "$CHANGED" | grep -qE "$pattern"
  if [ $? -eq 0 ]; then
    printf "%s matches provisioning test scope '%s'\n" "$(printf '%s\n' "$CHANGED" | grep -E "$pattern")" "$scope" >&2
  else
    return 1
  fi
}

selected=""
if [ "$ALL" = "1" ] || matches "$(yq '.full | join("|")' "$CONFIG")" "full"; then
  selected="$SCOPES"
else
  for scope in $SCOPES; do
    if matches "$(yq ".scopes.$scope.paths | join(\"|\")" "$CONFIG")" $scope; then
      selected="$selected $scope"
    fi
  done
fi

echo "resolved selected scopes: [$(echo "$selected" | xargs)]" >&2

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
    ]
  }
'
