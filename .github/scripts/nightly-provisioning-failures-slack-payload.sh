#!/usr/bin/env bash
#
# Builds the Slack webhook payload for nightly provisioning failures.
#
# Accepts a JSON object whose keys are branches and whose values contain branch metadata,
# a run URL, and an array of failing tests. Pass either a JSON file path or raw JSON as the
# first argument. Raw JSON may also be provided through FAILURE_BUNDLE.
#
# Optionally accepts an array of branch-keyed summary objects as the second argument or
# through FAILURE_SUMMARIES, for example: [{"main":{"summary":"Failure details"}}].
#
# Writes to GITHUB_OUTPUT:
#   any-failures=true|false
#   slack-payload=<JSON>   (only written when any-failures is true)

set -uo pipefail

GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"
NIGHTLY_TEST_FAILURE_LIMIT="${NIGHTLY_TEST_FAILURE_LIMIT:-3}"
BUNDLE_INPUT="${1:-${FAILURE_BUNDLE:-}}"
SUMMARY_INPUT="${2:-${FAILURE_SUMMARIES:-}}"

if [ -z "$BUNDLE_INPUT" ]; then
  echo "error: provide a failure bundle file or JSON through the first argument or FAILURE_BUNDLE" >&2
  exit 1
fi

bundle_filter='
  if type != "object"
    or any(.[];
      type != "object"
      or (.branch | type) != "string"
      or (.head_sha | type) != "string"
      or (.run_url | type) != "string"
      or (.failures | type) != "array"
    )
  then
    error("failure bundle must be an object whose values contain branch, head_sha, run_url, and failures")
  else
    .
  end
'

if [ -f "$BUNDLE_INPUT" ]; then
  complete_bundle=$(jq -ce "$bundle_filter" "$BUNDLE_INPUT") || exit 1
else
  complete_bundle=$(jq -ce "$bundle_filter" <<< "$BUNDLE_INPUT") || exit 1
fi

summary_filter='
  if length != 1
  then
    error("failure summaries must contain exactly one JSON array")
  else
    .[0]
    | if type != "array"
      or any(.[];
        type != "object"
        or any(.[]; type != "object" or (.summary | type) != "string")
      )
    then
      error("failure summaries must be an array of branch-keyed objects containing a string summary")
    else
      add // {}
    end
  end
'

summaries='{}'
if [ -n "$SUMMARY_INPUT" ]; then
  if [ -f "$SUMMARY_INPUT" ]; then
    summaries=$(jq -sce "$summary_filter" "$SUMMARY_INPUT") || summaries=''
  else
    summaries=$(jq -sce "$summary_filter" <<< "$SUMMARY_INPUT") || summaries=''
  fi
  if [ -z "$summaries" ]; then
    echo "::warning::Invalid failure analysis, posting without AI analysis" >&2
    summaries='{}'
  fi
fi

if [ "$(jq 'length' <<< "$complete_bundle")" -eq 0 ]; then
  echo "any-failures=false" >> "$GITHUB_OUTPUT"
  exit 0
fi

# We're using the slack block kit format,
# https://docs.slack.dev/block-kit/
containers=$(jq -cn \
  --argjson bundle "$complete_bundle" \
  --argjson summaries "$summaries" \
  '
  # One line per failing test case: `name` _(took Ns)_.
  def testline:
    (try (.time | tonumber | floor | tostring) catch "") as $t
    | "`" + .name + "`"
    + (if $t == "" then "" else " _(took " + $t + "s)_" end);

  # A container block for one branch: metadata, summary, detailed failures,
  # and the group of jobs that failed with no per-test results.
  def branch_container($idx; $key; $entry):
    $entry.branch as $branch
    | ($entry.failures | group_by(.job)) as $jobs
    | ($jobs | map(select(any(.[]; .name != null)))) as $detailed
    | ($jobs | map(select(all(.[]; .name == null)))) as $bare
    | {
        type: "container",
        block_id: ("container-" + $idx),
        icon: {
          type: "image",
          image_url: "https://avatars.githubusercontent.com/github",
          alt_text: ($branch + " branch icon")
        },
        title: {
          type: "plain_text",
          emoji: true,
          text: (($jobs | length | tostring) + " job(s) failed on " + $branch)
        },
        subtitle: {
          type: "plain_text",
          text: ("View Analysis of Final Attempt")
        },
        is_collapsible: true,
        default_collapsed: true,
        width: "full",
        child_blocks: (
          [
            {
              type: "section",
              block_id: ("container-meta-" + $idx),
              fields: [
                {
                  type: "mrkdwn",
                  text: (":github-blue: *Branch*\n`" + $branch + "`")
                },
                {
                  type: "mrkdwn",
                  text: (":hash: *Commit*\n`" + $entry.head_sha + "`")
                }
              ],
              accessory: {
                type: "button",
                text: {
                  type: "plain_text",
                  emoji: true,
                  text: ":mag: View run"
                },
                url: $entry.run_url,
                action_id: ("view-workflow-run-" + $idx),
                style: "primary"
              }
            },
            {
              type: "divider",
              block_id: ("container-div-" + $idx)
            }
          ]
          + (
            $summaries[$key].summary? as $summary
            | if $summary then
                [
                  {
                    type: "section",
                    block_id: ("container-summary-" + $idx),
                    text: {
                      type: "mrkdwn",
                      text: (":brain: *AI analysis*\n" + $summary)
                    }
                  },
                  {
                    type: "divider",
                    block_id: ("container-div-summary-" + $idx)
                  }
                ]
              else
                []
              end
          )
          + (
            if ($detailed | length) > 0 then
              [
                {
                  type: "section",
                  block_id: ("container-fails-" + $idx),
                  text: {
                    type: "mrkdwn",
                    text: (
                      ":fire: *The following tests failed*\n"
                      + ($detailed
                         | map(
                            ":bug: `" + .[0].job + "`\n"
                            + (map(testline) | join("\n"))
                          )
                         | join("\n\n"))
                    )
                  }
                }
              ]
            else
              []
            end
          )
          + (
            if ($bare | length) > 0 then
              [
                {
                  type: "section",
                  block_id: ("container-bare-" + $idx),
                  text: {
                    type: "mrkdwn",
                    text: (
                      ":warning: *The following jobs failed without producing per-test results*\n_This may mean they do not produce an XML bundle entry, or did not run at all._\n"
                      + ($bare | map(":bug: `" + .[0].job + "`") | join("\n"))
                    )
                  }
                }
              ]
            else
              []
            end
          )
        )
      };

  # to_entries preserves branch order; range over the entry count yields a
  # stable ordinal for labeling the blocks container-1, container-2, ...
  [
    $bundle
    | to_entries as $entries
    | range($entries | length)
    | . as $i
    | $entries[$i] as {key: $key, value: $entry}
    | branch_container((($i + 1) | tostring); $key; $entry)
  ]
  ')

slack_payload=$(jq -cn \
  --argjson containers "$containers" \
  --argjson bundle "$complete_bundle" \
  --arg failure_limit "$NIGHTLY_TEST_FAILURE_LIMIT" \
  --arg repo "${GITHUB_REPOSITORY:-rancher/rancher}" \
  '
   ($bundle
     | to_entries
     | map(
         .value.branch as $branch
         | (.value.failures | group_by(.job) | length | tostring) as $n
         | "• <https://github.com/" + $repo + "/tree/" + $branch + "|" + $branch + ">: "
             + $n + " job(s)"
       )
     | join("\n")) as $branch_lines
   | {
       blocks: ([
         {
           type: "header",
           block_id: "nightly-provisioning-failures-header",
           text: {
             type: "plain_text",
             emoji: true,
             text: (":moon: Nightly Provisioning Tests")
           }
         },
         {
           type: "section",
           block_id: "nightly-provisioning-failures-summary",
           text: {
             type: "mrkdwn",
             text: (
               ":fire: *Branches failing after " + $failure_limit + " attempts*\n"
               + $branch_lines
             )
           }
         },
         {
           type: "divider",
           block_id: "nightly-provisioning-failures-divider"
         }
       ] + $containers)
     }
  ')

echo "any-failures=true" >> "$GITHUB_OUTPUT"
echo "slack-payload=${slack_payload}" >> "$GITHUB_OUTPUT"
