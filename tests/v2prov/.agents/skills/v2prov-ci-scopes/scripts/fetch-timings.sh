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
#     shards: [{dist, regex, family, median_wall_s, median_test_step_s, samples}] }
#
# Only passing test cases with a non-zero time are counted (a timed out or
# never-started test reports 0). Shard times only come from successful jobs.
# Shard wall time includes runner boot, image load and Rancher startup;
# test_step_s is just the 'Run tests' step. family is the runner's EC2 family, read
# from the job's runs-on label, so c6id (NVMe) and c7i-flex (EBS) shards stay
# separate. Per-test timings are not split by family.
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
# require_value exits with a usage error when flag has no value after it. Without this,
# 'shift 2' on the last argument fails without shifting and the parse loop never ends.
require_value() {
  local flag="$1"
  local remaining="$2"
  if [ "$remaining" -lt 2 ]; then
    echo "error: $flag requires a value" >&2
    exit 2
  fi
}

while [ $# -gt 0 ]; do
  case "$1" in
    --branch)     require_value "$1" "$#"; BRANCH="$2"; shift 2 ;;
    --runs)       require_value "$1" "$#"; RUNS="$2"; shift 2 ;;
    --since-days) require_value "$1" "$#"; SINCE_DAYS="$2"; shift 2 ;;
    --repo)       require_value "$1" "$#"; REPO="$2"; shift 2 ;;
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

if ! BRANCH="$BRANCH" yq -e '.explicit[] | select(.name == "nightly") | .meta.branches[] | select(. == strenv(BRANCH))' "$CONFIG" >/dev/null 2>&1; then
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

# The nightly rows, so a shard's job name can be mapped back to the row it ran.
NIGHTLY_ROWS="$(yq -o=json '[.explicit[] | select(.name == "nightly") | .tests[]]' "$CONFIG" \
  | jq -c '[.[] | {dist: .V2PROV_TEST_DIST, regex: .V2PROV_TEST_RUN_REGEX}]')" \
  || { echo "error: failed to read nightly rows from $CONFIG" >&2; exit 2; }

TEST_SAMPLES="$TMP/tests.jsonl"
SHARD_SAMPLES="$TMP/shards.jsonl"
: > "$TEST_SAMPLES"
: > "$SHARD_SAMPLES"
USED_RUNS="[]"

for run_id in $(printf '%s' "$RUNS_JSON" | jq -r '.[].run_id'); do
  echo "run $run_id: downloading XML Results" >&2
  download_log="$TMP/download-$run_id.log"
  if ! gh run download "$run_id" -R "$REPO" -n "XML Results" -D "$TMP/$run_id" >"$download_log" 2>&1; then
    echo "warning: skipping run $run_id, could not download its 'XML Results' artifact: $(tr '\n' ' ' < "$download_log")" >&2
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
  #
  # GitHub truncates long job names with '...', which can cut the regex short. Each job's
  # regex is mapped back to a nightly row: an exact match first, then the only row on the
  # same dist whose regex starts with it. Jobs that map to no row, or to several, are
  # reported and left out rather than merged into the wrong shard.
  jobs_file="$TMP/jobs-$run_id.jsonl"
  if ! gh api --paginate "repos/$REPO/actions/runs/$run_id/jobs?per_page=100" \
      --jq '.jobs[] | select((.name | contains("Provisioning tests (")) and .conclusion == "success")' \
      > "$jobs_file"; then
    echo "warning: failed to list jobs for run $run_id" >&2
  fi
  jq -c --argjson run "$run_id" --argjson rows "$NIGHTLY_ROWS" '
      (.name | sub("^.*Provisioning tests \\("; "") | sub("\\)$"; "")) as $values
      # The cut can land right after a ", " separator, leaving a trailing comma or space.
      | ($values | sub(",? *(\\.\\.\\.|…)$"; "") | split(", ")) as $f
      | ([$f[] | select(test("^(k3s|rke2)$"))] | .[0]) as $dist
      | ([$f[] | select(test("^\\^?Test"))] | .[0]) as $seen
      # A row without a dist runs on both.
      | [$rows[] | select(.dist == $dist or .dist == null)] as $candidates
      | [$candidates[] | select(.regex == $seen)] as $exact
      | [$candidates[] | select($seen != null and (.regex | startswith($seen)))] as $prefixed
      | (if ($exact | length) == 1 then $exact else $prefixed end) as $matches
      | (.steps // [] | map(select(.name == "Run tests")) | .[0]) as $step
      | {name, dist: $dist, matches: ($matches | length),
         regex: (if ($matches | length) == 1 then $matches[0].regex else null end),
         run: $run,
         family: ([.labels[]? | capture("family=(?<f>[^,]+)").f] | .[0] // "unknown"),
         wall: ((.completed_at | fromdateiso8601) - (.started_at | fromdateiso8601)),
         test_step: (if $step and $step.started_at and $step.completed_at
                     then (($step.completed_at | fromdateiso8601) - ($step.started_at | fromdateiso8601))
                     else null end)}' "$jobs_file" > "$TMP/mapped-$run_id.jsonl" \
    || echo "warning: failed to parse jobs for run $run_id" >&2

  jq -r 'select(.regex == null)
      | if .matches > 1 then "warning: run \(.run): job matches \(.matches) nightly rows, skipping it: \(.name)"
        else "warning: run \(.run): job matches no nightly row in this config, skipping it: \(.name)" end' \
    "$TMP/mapped-$run_id.jsonl" >&2
  jq -c 'select(.regex != null) | {dist, regex, family, run, wall, test_step}' "$TMP/mapped-$run_id.jsonl" >> "$SHARD_SAMPLES"

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
    shards: ($shards | group_by([.dist, .regex, .family])
      | map({dist: .[0].dist, regex: .[0].regex, family: .[0].family,
             median_wall_s: (map(.wall) | median),
             median_test_step_s: (map(.test_step | select(. != null)) | median),
             samples: length})
      | sort_by(-.median_wall_s))
  }')" || { echo "error: failed to aggregate timings" >&2; exit 2; }

printf '%s' "$RESULT" | jq -r '
  "branch \(.branch), runs: \(.runs | map("\(.run_id) (\(.created_at[:10]), \(.conclusion), \(.head_sha[:9]))") | join(", "))",
  "\(.tests | length) test timings, \(.shards | length) shard timings",
  "slowest shards (wall / test step, minutes):",
  (.shards[:8][] | "  \(.dist)\t\(.family)\t\(.median_wall_s / 60 | floor) / \(if .median_test_step_s then (.median_test_step_s / 60 | floor) else "?" end)\t\(.regex)")' >&2

printf '%s\n' "$RESULT"
