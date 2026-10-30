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
# Writes to GITHUB_OUTPUT:
#   any-failures=true|false
#   slack-payload=<JSON>   (only written when any-failures is true)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG_FILE="${SCOPES_CONFIG:-$SCRIPT_DIR/provisioning-test-scopes.yaml}"
WORKFLOW_FILE="${NIGHTLY_WORKFLOW_FILE:-nightly-provisioning-tests.yml}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-rancher/rancher}"
GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"
HOURS_AGO="${HOURS_AGO:-9}"
NIGHTLY_TEST_FAILURE_LIMIT=${NIGHTLY_TEST_FAILURE_LIMIT:-3}
BOT_ACTOR="rancher-rancher-nightly-dispatcher[bot]"

log() {
  echo "[nightly-prov-report] $*" >&2
}

# Supports both the macos and linux `date` command
# so that this script can be run locally in any env
if date -v-"$HOURS_AGO"H >/dev/null 2>&1; then
  FORMATTED_HOURS_AGO="$(date -u -v-"$HOURS_AGO"H +%Y-%m-%dT%H:%M:%S)"
else
  FORMATTED_HOURS_AGO="$(date -u -d "$HOURS_AGO hours ago" +%Y-%m-%dT%H:%M:%S)"
fi

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

all_bundles=()

for branch in $TARGET_BRANCHES; do
    log "Checking $branch..."

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
    run_details=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${candidate_id}")
    triggering_actor=$(jq -r '.triggering_actor.login' <<< "$run_details")
    if [[ "$triggering_actor" = "${BOT_ACTOR}" || "$triggering_actor" = "github-actions[bot]" ]]; then
      run_id="$candidate_id"
      head_sha=$(jq -er '.head_sha | select(type == "string" and length > 0)' <<< "$run_details") || exit 1
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
      gh api -H "Accept: application/vnd.github+json" \
        "repos/${GITHUB_REPOSITORY}/actions/artifacts/${artifact_id}/zip" > "$zip_file"
      unzip -oq "$zip_file" -d "$run_dir"
      rm -rf "$artifact_dir" "$zip_file"
      log "artifact $artifact_id downloaded"
    fi

    # JSON array of failing job names (e.g. "k3s, ^Test_(General|Provisioning|Fleet)_.*$").
    # The jobs endpoint is paginated, and --jq runs once per page, so emit one JSON string
    # per job and slurp them into a single array.
    failing_job_names=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/runs/${run_id}/jobs?per_page=100" \
    --jq '.jobs[] | select(.conclusion == "failure") | .name | @json' | jq -sc '.') || {
      log "error: failed to list jobs for run ${run_id}"
      exit 1
    }

    # One time conversion of xml -> json
    files=()
    for file in "$run_dir"/*; do
    if [ -f "$file" ]; then
      if [[ "$file" == *.xml ]]; then
        yq --xml-attribute-prefix="" -p=xml -o=json "$file" > "${file%.xml}.json"
        files+=("${file%.xml}.json")
      fi
    fi
    done

    log "building bundle.json"

    # No XML reports were published (artifact missing, expired, or the run crashed
    # before any report was written). Fall back to a bundle containing only the
    # failing job names so that we always produce output.
    if [ "${#files[@]}" -eq 0 ]; then
      log "no XML reports found for run ${run_id} on branch ${branch}, falling back to failing job names only"
      failing_tests=$(jq -n \
        --argjson jobs "$failing_job_names" \
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
        }')

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
    failing_tests=$(jq -n \
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
      }' "${files[@]}")

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
echo "any-failures=$any_failures" >> "$GITHUB_OUTPUT"
