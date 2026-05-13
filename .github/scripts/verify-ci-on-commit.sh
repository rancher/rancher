#!/usr/bin/env bash
set -euo pipefail

# Verifies that CI passed on the pull request that merged COMMIT_SHA into
# the target branch. Fails if no merged PR is found, since all commits to
# main and release branches must arrive via a pull request.
#
# Required environment variables:
#   COMMIT_SHA  - the commit to verify
#   REPO        - the GitHub repository in "owner/repo" form
#   GH_TOKEN    - a token with pull-requests:read and checks:read

: "${COMMIT_SHA:?COMMIT_SHA is required}"
: "${REPO:?REPO is required}"
: "${GH_TOKEN:?GH_TOKEN is required}"

echo "Verifying CI checks for commit ${COMMIT_SHA} in ${REPO}"

PR_NUMBER=$(gh api \
    "repos/${REPO}/commits/${COMMIT_SHA}/pulls" \
    --jq "[.[] | select(.merge_commit_sha == \"${COMMIT_SHA}\")] | first | .number // empty")

if [[ -z "${PR_NUMBER:-}" ]]; then
    echo "ERROR: no merged PR found for ${COMMIT_SHA}" >&2
    exit 1
fi

echo "Found PR #${PR_NUMBER}, checking required CI..."
gh pr checks "${PR_NUMBER}" --repo "${REPO}" --required
