#!/usr/bin/env bash
# =============================================================================
#
# Collects per-test and per-shard durations for v2prov provisioning tests from
# recent nightly runs on ONE branch.
#
# Only nightly runs are used: they test merged code. PR runs can reflect code
# that never merges, and other branches run different logic, so neither is a
# valid stand-in. If the target branch has no nightly data, this exits 3 and
# the caller should fall back to the node-count cost model.
#
# Usage:
#   fetch-timings.sh --branch <branch> [--runs N] [--since-days D] [--repo owner/name]
#
#   --branch      required, must be listed in explicit[nightly].meta.branches
#   --runs        newest completed nightly runs to sample (default 3)
#   --since-days  ignore runs older than this (default 14)
#   --repo        GitHub repo to query (default rancher/rancher)
#
# Output (stdout, JSON):
#   { branch, runs:   [{run_id, head_sha, created_at, conclusion, url}],
#     tests:  [{name, dist, median_s, samples}],
#     shards: [{dist, regex, median_wall_s, median_test_step_s, samples}] }
#
# Only passing test cases with a non-zero time are counted (a timed out or
# never-started test reports 0). Shard times only come from successful jobs.
# Shard wall time includes runner boot, image load and Rancher startup;
# test_step_s is just the 'Run tests' step.
#
# Exit status: 0 ok, 2 usage/prerequisite error, 3 no nightly data for branch.
#
# =============================================================================
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "error: not inside a git checkout" >&2; exit 2; }
CONFIG="${SCOPES_CONFIG:-$ROOT/.github/scripts/provisioning-test-scopes.yaml}"
WORKFLOW=nightly-provisioning-tests.yml

BRANCH=""
RUNS=3
SINCE_DAYS=14
REPO=rancher/rancher
while [ $# -gt 0 ]; do
  case "$1" in
    --branch)     BRANCH="${2:-}"; shift 2 ;;
    --runs)       RUNS="${2:-}"; shift 2 ;;
    --since-days) SINCE_DAYS="${2:-}"; shift 2 ;;
    --repo)       REPO="${2:-}"; shift 2 ;;
    -h|--help)    sed -n '4,31p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)            echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done

test -n "$BRANCH"                        || { echo "error: --branch is required" >&2; exit 2; }
[[ "$RUNS" =~ ^[1-9][0-9]*$ ]]           || { echo "error: --runs must be a positive integer" >&2; exit 2; }
[[ "$SINCE_DAYS" =~ ^[1-9][0-9]*$ ]]     || { echo "error: --since-days must be a positive integer" >&2; exit 2; }
test -f "$CONFIG"                        || { echo "error: config not found: $CONFIG" >&2; exit 2; }
for bin in gh yq jq; do
  command -v "$bin" >/dev/null           || { echo "error: $bin is required" >&2; exit 2; }
done
gh auth status >/dev/null 2>&1           || { echo "error: gh is not authenticated (run: gh auth login)" >&2; exit 2; }

if ! yq -e ".explicit[] | select(.name == \"nightly\") | .meta.branches[] | select(. == \"$BRANCH\")" "$CONFIG" >/dev/null 2>&1; then
  echo "no nightly timing data for $BRANCH: branch is not in explicit[nightly].meta.branches" >&2
  exit 3
fi

if SINCE="$(date -u -v-"${SINCE_DAYS}"d +%Y-%m-%d 2>/dev/null)"; then :; else
  SINCE="$(date -u -d "${SINCE_DAYS} days ago" +%Y-%m-%d)" || { echo "error: cannot compute --since-days date" >&2; exit 2; }
fi

# Nightly is dispatched with --ref <branch>, so head_branch identifies the branch
# under test. The query already filters on it; the select is a second guard.
RUNS_JSON="$(gh api -X GET "repos/$REPO/actions/workflows/$WORKFLOW/runs" \
    -f branch="$BRANCH" -f event=workflow_dispatch -f status=completed \
    -f created=">=$SINCE" -f per_page=50 \
  | jq --arg b "$BRANCH" --argjson n "$RUNS" '
      [.workflow_runs[]
        | select(.head_branch == $b and (.conclusion == "success" or .conclusion == "failure"))
        | {run_id: .id, head_sha, created_at, conclusion, url: .html_url}]
      | sort_by(.created_at) | reverse | .[:$n]')" \
  || { echo "error: failed to list $WORKFLOW runs for $BRANCH in $REPO" >&2; exit 2; }

if [ "$(printf '%s' "$RUNS_JSON" | jq 'length')" -eq 0 ]; then
  echo "no nightly timing data for $BRANCH: no completed $WORKFLOW runs since $SINCE in $REPO" >&2
  exit 3
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

TEST_SAMPLES="$TMP/tests.jsonl"
SHARD_SAMPLES="$TMP/shards.jsonl"
: > "$TEST_SAMPLES"
: > "$SHARD_SAMPLES"
USED_RUNS="[]"

for run_id in $(printf '%s' "$RUNS_JSON" | jq -r '.[].run_id'); do
  echo "run $run_id: downloading XML Results" >&2
  if ! gh run download "$run_id" -R "$REPO" -n "XML Results" -D "$TMP/$run_id" >/dev/null 2>&1; then
    echo "warning: run $run_id has no downloadable 'XML Results' artifact (expired or never uploaded), skipping" >&2
    continue
  fi

  for xml in "$TMP/$run_id"/report-*.xml; do
    test -f "$xml" || continue
    base="$(basename "$xml" .xml)"
    dist="${base#report-}"; dist="${dist%%-*}"
    yq -p xml -o json '.' "$xml" | jq -c --arg dist "$dist" --argjson run "$run_id" '
      .testsuites.testsuite // [] | (if type == "array" then .[] else . end)
      | .testcase // empty | (if type == "array" then .[] else . end)
      | select((.["+@name"] | contains("/") | not)
               and (has("failure") or has("error") or has("skipped") | not))
      | (.["+@time"] | tonumber) as $t | select($t > 0)
      | {name: .["+@name"], dist: $dist, time: $t, run: $run}' >> "$TEST_SAMPLES" \
      || echo "warning: failed to parse $(basename "$xml") from run $run_id" >&2
  done

  # Job names look like '<caller> / Provisioning tests (<values>)', where <values> are the
  # matrix row's values in the order the resolver emits them. Rows pass their YAML keys
  # through in their own order and defaults are appended, so the position of dist and
  # regex varies between rows; pick them out by content instead. Regexes are restricted
  # to [][A-Za-z0-9_()|.^$*?], so they never contain ', '.
  if ! gh api --paginate "repos/$REPO/actions/runs/$run_id/jobs?per_page=100" \
      --jq '.jobs[] | select((.name | contains("Provisioning tests (")) and .conclusion == "success")' \
    | jq -c --argjson run "$run_id" '
        (.name | sub("^.*Provisioning tests \\("; "") | sub("\\)$"; "") | split(", ")) as $f
        | ([$f[] | select(test("^(k3s|rke2)$"))] | .[0]) as $dist
        | ([$f[] | select(test("^\\^?Test"))] | .[0]) as $regex
        | select($dist != null and $regex != null)
        | (.steps // [] | map(select(.name == "Run tests")) | .[0]) as $step
        | {dist: $dist, regex: $regex, run: $run,
           wall: ((.completed_at | fromdateiso8601) - (.started_at | fromdateiso8601)),
           test_step: (if $step and $step.started_at and $step.completed_at
                       then (($step.completed_at | fromdateiso8601) - ($step.started_at | fromdateiso8601))
                       else null end)}' >> "$SHARD_SAMPLES"; then
    echo "warning: failed to list jobs for run $run_id" >&2
  fi

  USED_RUNS="$(printf '%s' "$USED_RUNS" | jq --argjson all "$RUNS_JSON" --argjson id "$run_id" '. + [$all[] | select(.run_id == $id)]')"
done

if [ "$(printf '%s' "$USED_RUNS" | jq 'length')" -eq 0 ]; then
  echo "no nightly timing data for $BRANCH: none of the sampled runs had usable artifacts" >&2
  exit 3
fi

RESULT="$(jq -n \
  --arg branch "$BRANCH" \
  --argjson runs "$USED_RUNS" \
  --slurpfile tests "$TEST_SAMPLES" \
  --slurpfile shards "$SHARD_SAMPLES" '
  def median: sort | length as $n
    | if $n == 0 then null
      elif $n % 2 == 1 then .[($n - 1) / 2]
      else (.[$n / 2 - 1] + .[$n / 2]) / 2 end
    | if . == null then null else (. * 10 | round / 10) end;
  {
    branch: $branch,
    runs: $runs,
    tests: ($tests | group_by([.name, .dist])
      | map({name: .[0].name, dist: .[0].dist, median_s: (map(.time) | median), samples: length})
      | sort_by(-.median_s)),
    shards: ($shards | group_by([.dist, .regex])
      | map({dist: .[0].dist, regex: .[0].regex,
             median_wall_s: (map(.wall) | median),
             median_test_step_s: (map(.test_step | select(. != null)) | median),
             samples: length})
      | sort_by(-.median_wall_s))
  }')" || { echo "error: failed to aggregate timings" >&2; exit 2; }

printf '%s' "$RESULT" | jq -r '
  "branch \(.branch), runs: \(.runs | map("\(.run_id) (\(.created_at[:10]), \(.conclusion), \(.head_sha[:9]))") | join(", "))",
  "\(.tests | length) test timings, \(.shards | length) shard timings",
  "slowest shards (wall / test step, minutes):",
  (.shards[:8][] | "  \(.dist)\t\(.median_wall_s / 60 | floor) / \(if .median_test_step_s then (.median_test_step_s / 60 | floor) else "?" end)\t\(.regex)")' >&2

printf '%s\n' "$RESULT"
