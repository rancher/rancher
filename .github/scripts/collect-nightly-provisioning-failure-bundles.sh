#!/usr/bin/env bash
#
# Iterates over the most recent nightly provisioning tests workflows (past 9 hours) and
# identifies which ones have failed. For each failing branch, the latest test output XML file
# is downloaded and used to create a single failure bundle.json file. We check branches listed in the provisioning-test-scopes.yaml's
# explicit.nightly.meta.branches list.
#
# For each branch, only a run's 3rd attempt (by default) counts as a real failure, and only if
# that run was started by the nightly scheduler itself (github-actions[bot]), not someone
# manually re-running it.
#
# A failing branch is never dropped because of a collection error. If an artifact or job list
# can't be fetched or parsed, the branch is still bundled with whatever could be collected
# (down to a placeholder job name).
#
# Separately, a run that started more than STUCK_RUN_THRESHOLD_HOURS ago and has still not
# completed is reported as still executing. This typically indicates a runner scheduling issue.
# These runs are written to running-bundle.json in the same format as bundle.json (plus "running"
# and "created_at"), kept apart from bundle.json so the failure analysis never sees them.
#
# Writes bundle.json and running-bundle.json, and to GITHUB_OUTPUT:
#   any-failures=true|false
#   any-running=true|false

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="${SCOPES_CONFIG:-$SCRIPT_DIR/provisioning-test-scopes.yaml}"
WORKFLOW_FILE="${NIGHTLY_WORKFLOW_FILE:-nightly-provisioning-tests.yml}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-rancher/rancher}"
GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"
HOURS_AGO="${HOURS_AGO:-9}"
STUCK_RUN_THRESHOLD_HOURS="${STUCK_RUN_THRESHOLD_HOURS:-4}"
NIGHTLY_TEST_FAILURE_LIMIT=${NIGHTLY_TEST_FAILURE_LIMIT:-3}
BOT_ACTOR="rancher-rancher-nightly-dispatcher[bot]"

log() {
  echo "[nightly-prov-report] $*" >&2
}

# Prints the UTC timestamp from N hours ago. Supports both the macos
# and linux `date` command so that this script can be run locally in any env
hours_ago_timestamp() {
  local hours="$1"
  if date -v-"$hours"H >/dev/null 2>&1; then
    date -u -v-"$hours"H +%Y-%m-%dT%H:%M:%S
  else
    date -u -d "$hours hours ago" +%Y-%m-%dT%H:%M:%S
  fi
}

FORMATTED_HOURS_AGO="$(hours_ago_timestamp "$HOURS_AGO")"
FORMATTED_STUCK_CUTOFF="$(hours_ago_timestamp "$STUCK_RUN_THRESHOLD_HOURS")"

# Only runs started by the nightly scheduler count, so we don't
# alert on human-run attempts.
is_nightly_actor() {
  local actor="$1"
  if [ "$actor" = "$BOT_ACTOR" ] || [ "$actor" = "github-actions[bot]" ]; then
    return 0
  fi
  return 1
}

# Prints a bundle entry that only lists the failing job names, used when
# per-test results are unavailable.
job_names_only_bundle() {
  local branch="$1"
  local run_url="$2"
  local head_sha="$3"
  local jobs_json="$4"
  jq -n \
    --argjson jobs "$jobs_json" \
    --arg branch "$branch" \
    --arg run_url "$run_url" \
    --arg head_sha "$head_sha" \
    '{
      branch: $branch,
      run_url: $run_url,
      head_sha: $head_sha,
      failures: [
        $jobs[]
        | { job: ., name: null, classname: null, time: null, failure: null }
      ]
    }'
}

# Adds an entry to $running_bundle for every nightly run on the branch that was created between
# HOURS_AGO and STUCK_RUN_THRESHOLD_HOURS ago and has not completed. Failing to look
# is logged, but never fails the script.
collect_unfinished_runs() {
  local branch="$1"
  local unfinished_ids
  if ! unfinished_ids=$(gh run list \
      --repo "$GITHUB_REPOSITORY" \
      --workflow="$WORKFLOW_FILE" \
      --branch "$branch" \
      --limit 50 \
      --json databaseId,status \
      --created "${FORMATTED_HOURS_AGO}..${FORMATTED_STUCK_CUTOFF}" \
      --jq '.[] | select(.status != "completed") | .databaseId'); then
    log "error: failed to list unfinished runs on branch $branch"
    return 0
  fi

  local unfinished_id
  for unfinished_id in $unfinished_ids; do
    local run_details
    if ! run_details=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${unfinished_id}"); then
      log "error: failed to fetch details of unfinished run ${unfinished_id}, skipping it"
      continue
    fi

    local actor
    actor=$(jq -r '.triggering_actor.login' <<< "$run_details")
    if ! is_nightly_actor "$actor"; then
      continue
    fi

    # Names of the jobs that have not finished yet.
    local unfinished_jobs
    if ! unfinished_jobs=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/runs/${unfinished_id}/jobs?per_page=100" \
        --jq '.jobs[] | select(.status != "completed") | .name | @json' | jq -sc '.'); then
      log "error: failed to list jobs for unfinished run ${unfinished_id}"
      unfinished_jobs='[]'
    fi
    # The slack payload lists a run's jobs, so a run always needs at least one.
    if [ "$unfinished_jobs" = "[]" ]; then
      unfinished_jobs='["unknown (no unfinished jobs found)"]'
    fi

    local unfinished_sha
    unfinished_sha=$(jq -r '.head_sha // "unknown"' <<< "$run_details")
    local unfinished_created_at
    unfinished_created_at=$(jq -r '.created_at // "unknown"' <<< "$run_details")

    local entry
    if ! entry=$(jq -n \
        --argjson jobs "$unfinished_jobs" \
        --arg branch "$branch" \
        --arg run_url "https://github.com/${GITHUB_REPOSITORY}/actions/runs/${unfinished_id}" \
        --arg head_sha "$unfinished_sha" \
        --arg created_at "$unfinished_created_at" \
        '{
          branch: $branch,
          run_url: $run_url,
          head_sha: $head_sha,
          running: true,
          created_at: $created_at,
          failures: [
            $jobs[]
            | { job: ., name: null, classname: null, time: null, failure: null }
          ]
        }'); then
      log "error: failed to build entry for unfinished run ${unfinished_id}"
      continue
    fi

    log "Run $unfinished_id on branch $branch is still executing"
    running_bundle=$(jq -c --arg key "${branch}-${unfinished_id}" --argjson entry "$entry" \
      '. + {($key): $entry}' <<< "$running_bundle")
    any_running="true"
  done
}

log "Checking for Nightly Failures Since $FORMATTED_HOURS_AGO"

if [ ! -f "$CONFIG_FILE" ]; then
  log "error: config not found: $CONFIG_FILE"
  exit 1
fi

if ! command -v yq >/dev/null; then
  log "error: yq (v4) is required"
  exit 1
fi

# Space-separated list of branches to check, e.g. "main release/v2.15 release/v2.14"
TARGET_BRANCHES=$(yq -o=json '.explicit[] | select(.name == "nightly") | .meta.branches' "$CONFIG_FILE" | jq -r '.[]' | paste -sd ' ' - )

log "Checking target Branches [$TARGET_BRANCHES]"
any_failures="false"
any_running="false"
running_bundle="{}"

all_bundles=()

for branch in $TARGET_BRANCHES; do
    log "Checking $branch..."

    collect_unfinished_runs "$branch"

    # IDs of completed runs, on this branch, in the last $HOURS_AGO hours, that failed.
    failing_run_ids=$(gh run list \
        --repo "$GITHUB_REPOSITORY" \
        --workflow="$WORKFLOW_FILE" \
        --branch "$branch" \
        --status completed \
        --limit 50 \
        --json databaseId,attempt,conclusion \
        --created "${FORMATTED_HOURS_AGO}..*" \
        --jq "[.[] | select(.conclusion == \"failure\" and .attempt == $NIGHTLY_TEST_FAILURE_LIMIT)] | .[].databaseId")

    # Limit results to workflow runs initiated by bots, so we don't
    # alert on human-run attempts
    head_sha=""
    run_id=""
    for candidate_id in $failing_run_ids; do
    log "Checking if $candidate_id was triggered by the correct bot actor"
    if ! run_details=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${candidate_id}"); then
      log "error: failed to fetch details of run ${candidate_id}, skipping it"
      continue
    fi
    triggering_actor=$(jq -r '.triggering_actor.login' <<< "$run_details")
    if is_nightly_actor "$triggering_actor"; then
      run_id="$candidate_id"
      head_sha=$(jq -r '.head_sha | select(type == "string" and length > 0) // "unknown"' <<< "$run_details")
      break
    fi
    done

    if [ -z "$run_id" ]; then
        log "Found no failing run_ids in the past $HOURS_AGO hours for branch $branch"
        continue
    fi

    log "Nightly run $run_id failed on branch $branch after $NIGHTLY_TEST_FAILURE_LIMIT attempt(s)"

    any_failures="true"
    run_url="https://github.com/${GITHUB_REPOSITORY}/actions/runs/${run_id}"
    run_dir="./${branch}-${run_id}-artifacts"
    mkdir -p $run_dir

    # Download the raw XML output of the failing run. This artifact includes the specific tests
    # and logs for failing tests. There isn't a way to associate an artifact with a specific test
    # attempt, so we are stuck just looking at the most recent one.
    artifact_id="$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${run_id}/artifacts?per_page=100" \
    --jq '[.artifacts[] | select(.expired == false and .name == "XML Results")]
          | sort_by(.created_at) | last | .id // empty')"

    artifact_dir="${run_dir}/set-${run_id}"
    if [ -z "$artifact_id" ]; then
      log "no XML artifacts published for run ${run_id} on branch ${branch}. Bundling failing job names only"
    else
      zip_file="${artifact_dir}.zip"
      mkdir -p "$artifact_dir"
      if gh api -H "Accept: application/vnd.github+json" \
        "repos/${GITHUB_REPOSITORY}/actions/artifacts/${artifact_id}/zip" > "$zip_file" \
        && unzip -oq "$zip_file" -d "$run_dir"; then
        log "artifact $artifact_id downloaded"
      else
        log "error: failed to download or extract artifact $artifact_id, bundling failing job names only"
      fi
      rm -rf "$artifact_dir" "$zip_file"
    fi

    # JSON array of failing job names (e.g. "k3s, ^Test_(General|Provisioning|Fleet)_.*$").
    # The jobs endpoint is paginated, and --jq runs once per page, so emit one JSON string
    # per job and slurp them into a single array.
    if ! failing_job_names=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/runs/${run_id}/jobs?per_page=100" \
    --jq '.jobs[] | select(.conclusion == "failure") | .name | @json' | jq -sc '.'); then
      log "error: failed to list jobs for run ${run_id}, bundling a placeholder job name"
      failing_job_names='["unknown (failed to list jobs)"]'
    fi

    # One time conversion of xml -> json
    files=()
    for file in "$run_dir"/*; do
    if [ -f "$file" ]; then
      if [[ "$file" == *.xml ]]; then
        if yq --xml-attribute-prefix="" -p=xml -o=json "$file" > "${file%.xml}.json"; then
          files+=("${file%.xml}.json")
        else
          log "error: failed to convert $file to json, skipping it"
        fi
      fi
    fi
    done

    log "building bundle.json"

    # No XML reports were published (artifact missing, expired, or the run crashed
    # before any report was written). Fall back to a bundle containing only the
    # failing job names so that we always produce output.
    if [ "${#files[@]}" -eq 0 ]; then
      log "no XML reports found for run ${run_id} on branch ${branch}, falling back to failing job names only"
      failing_tests=$(job_names_only_bundle "$branch" "$run_url" "$head_sha" "$failing_job_names")

      bundle_file="${run_dir}/${branch//\//-}.json"
      echo "$failing_tests" > "$bundle_file"
      all_bundles+=("$bundle_file")
      log "done writing bundle.json for branch ${branch}"
      continue
    fi

    # For all json files, look for failing test cases and associate them
    # with the failing job (via file naming conventions). Failing jobs that have
    # no matching test case entry (no report uploaded, crashed before tests ran,
    # integration tests, etc.) get a placeholder entry, so that no failing job is ever dropped.
    if ! failing_tests=$(jq -n \
    --argjson jobs "$failing_job_names" \
    --arg branch "$branch" \
    --arg run_url "$run_url" \
    --arg head_sha "$head_sha" \
    '
    # A lone <testsuite>/<testcase> element without siblings is emitted as a
    # bare object (not an array) by the xml conversion, so wrap it before
    # iterating.
    def asarray: if type == "array" then . else [.] end;

    # The xml conversion emits element text under a "+content" key, we need to rename it.
    def clean_content:
      if type == "object"
      then with_entries(if .key == "+content" then .key = "content" else . end)
      else .
      end;

    # The failing job a report belongs to is the first failing job whose name
    # mentions both the distribution and the suite, else "dist, suite".
    def matched_job($report):
      (
        [
          $jobs[]
          | select(
              (ascii_downcase | contains($report.dist | ascii_downcase))
              and (sub("Test"; "") | gsub("[()|_^.*$]"; "") | contains($report.suite))
            )
        ]
        | first
      )
      // ($report.dist + ", " + $report.suite);

    # One entry per failing test case found in the downloaded reports.
    [
      inputs
      | (input_filename | split("/")[-1]
         | capture("^report-(?<dist>[^-]+)-(?<suite>.*)\\.json$")) as $report
      | ((.testsuites.testsuite // empty) | asarray)[]
      | select(.failures != "0" or .errors != "0")
      | ((.testcase // empty) | asarray)[]
      | select(.failure != null or .error != null)
      | {
          job: matched_job($report),
          name,
          classname,
          time,
          failure: .failure | clean_content,
          error: .error | clean_content
        }
    ] as $tests
    | ($jobs - ($tests | map(.job))) as $uncovered
    | {
        branch: $branch,
        run_url: $run_url,
        head_sha: $head_sha,
        failures: (
          $tests
          + [
              $uncovered[]
              | { job: ., name: null, classname: null, time: null, failure: null }
            ]
        )
      }' "${files[@]}"); then
      log "error: failed to build per-test failures for run ${run_id}, falling back to failing job names only"
      failing_tests=$(job_names_only_bundle "$branch" "$run_url" "$head_sha" "$failing_job_names")
    fi

    bundle_file="${run_dir}/${branch//\//-}.json"
    echo "$failing_tests" > "$bundle_file"
    all_bundles+=("$bundle_file")
done

# Produce a final JSON array where each entry is named
# after the branch the tests failed on.
# e.g.
# {
#    "main": {
#      "branch": "main",
#      "run_url": "https://github.com/rancher/rancher/actions/runs/35821850565",
#      "head_sha": "1a7ed5e31da56631a05a5e8cbf44c428d0945068",
#      "failures": [
#        ...
#       ]
#     }
# }
if [ "${#all_bundles[@]}" -eq 0 ]; then
  complete_bundle="{}"
else
  complete_bundle=$(jq -n '
    reduce inputs as $bundle (
      {};
      .[(input_filename | split("/")[-1] | sub("\\.[^.]+$"; ""))] = $bundle
    )
  ' "${all_bundles[@]}")
fi

echo "$complete_bundle" > bundle.json
echo "$running_bundle" > running-bundle.json
echo "any-failures=$any_failures" >> "$GITHUB_OUTPUT"
echo "any-running=$any_running" >> "$GITHUB_OUTPUT"
