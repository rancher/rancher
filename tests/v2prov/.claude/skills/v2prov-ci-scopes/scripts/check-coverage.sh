#!/usr/bin/env bash
# =============================================================================
#
# Reports how every top-level v2prov test maps onto the rows in
# provisioning-test-scopes.yaml, and flags configuration problems.
#
# Usage:
#   check-coverage.sh                 full placement table + problems
#   check-coverage.sh --test <name>   placement for one test + problems
#   check-coverage.sh --problems      problems only (stable, sorted, diffable)
#   check-coverage.sh --json          raw analysis as JSON
#
# SCOPES_CONFIG overrides the config path, so a proposed edit can be checked
# from a temp copy before it is written to the real file.
#
# Dist skips, t.Parallel() and env/feature gates are detected statically from
# the test body and the same-package helpers it calls (up to 3 levels deep).
# This is a heuristic: always read the test before trusting a skip flag.
#
# Exit status: 0 when no problems are found, 1 when problems are found,
# 2 on usage or prerequisite errors.
#
# =============================================================================
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "error: not inside a git checkout" >&2; exit 2; }
CONFIG="${SCOPES_CONFIG:-$ROOT/.github/scripts/provisioning-test-scopes.yaml}"
TESTS_DIR="$ROOT/tests/v2prov/tests"

# Mirrors the whitelist in scripts/provisioning-tests and the cap in
# .github/workflows/provisioning-tests.yml.
REGEX_WHITELIST='^[][A-Za-z0-9_()|.^$*?]+$'
MATRIX_CAP=25

MODE=table
TEST_NAME=""
while [ $# -gt 0 ]; do
  case "$1" in
    --test)     TEST_NAME="${2:-}"; MODE=test; shift 2 ;;
    --problems) MODE=problems; shift ;;
    --json)     MODE=json; shift ;;
    -h|--help)  sed -n '4,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)          echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ "$MODE" = test ] && [ -z "$TEST_NAME" ]; then
  echo "error: --test requires a test name" >&2
  exit 2
fi

test -f "$CONFIG"        || { echo "error: config not found: $CONFIG" >&2; exit 2; }
command -v yq >/dev/null || { echo "error: yq (v4) is required" >&2; exit 2; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 2; }

# extract_funcs prints one JSON object per Go func under tests/v2prov/tests with
# the signals needed to classify it. A dist/gate condition only counts if a
# t.Skip follows within 3 lines, which filters out unrelated DIST comparisons.
extract_funcs() {
  find "$TESTS_DIR" -name '*.go' | sort | while IFS= read -r f; do
    awk -v rel="${f#"$ROOT"/}" '
      function flush() {
        if (name == "") return
        printf "{\"pkg\":\"%s\",\"name\":\"%s\",\"file\":\"%s\",\"line\":%d,\"parallel\":%s,\"skip_rke2\":%s,\"skip_k3s\":%s,\"gates\":[%s],\"calls\":[%s]}\n",
          pkg, name, rel, line, par, sr, sk, gates, calls
        name = ""
      }
      function addgate(g) {
        if (index(gates, "\"" g "\"") == 0) gates = gates (gates == "" ? "" : ",") "\"" g "\""
      }
      BEGIN { pkg = rel; sub(/\/[^\/]*$/, "", pkg) }
      /^func / {
        flush()
        s = $0
        sub(/^func /, "", s)
        if (s ~ /^\(/) sub(/^\([^)]*\) */, "", s)
        n = s
        sub(/[^A-Za-z0-9_].*$/, "", n)
        name = n; line = FNR
        par = "false"; sr = "false"; sk = "false"; gates = ""; calls = ""
        pending = ""; ttl = 0
        split("", seen)
        next
      }
      name != "" {
        if ($0 ~ /t\.Parallel\(\)/) par = "true"

        if ($0 ~ /"DIST"\)+ *== *"rke2"/ || $0 ~ /== *capr\.RuntimeRKE2/ || $0 ~ /!= *capr\.RuntimeK3S/) { pending = "rke2"; ttl = 4 }
        else if ($0 ~ /"DIST"\)+ *!= *"rke2"/ || $0 ~ /!= *capr\.RuntimeRKE2/ || $0 ~ /== *capr\.RuntimeK3S/ || $0 ~ /"DIST"\)+ *== *"k3s"/) { pending = "k3s"; ttl = 4 }
        else if ($0 ~ /V2PROV_TEST_CAPRKE2/) { pending = "gate:V2PROV_TEST_CAPRKE2"; ttl = 4 }
        else if ($0 ~ /SOME_K8S_VERSION_PREV|previousK8sVersion == ""/) { pending = "gate:SOME_K8S_VERSION_PREV"; ttl = 4 }
        else if ($0 ~ /features\.[A-Za-z0-9]+\.Enabled\(\)/) {
          g = $0; sub(/.*features\./, "", g); sub(/\.Enabled.*/, "", g)
          pending = "gate:feature:" g; ttl = 4
        }
        else if ($0 ~ /pod CIDR/) { pending = "gate:dual-stack-host"; ttl = 1 }

        if (pending != "" && $0 ~ /t\.Skip/) {
          if (pending == "rke2") sr = "true"
          else if (pending == "k3s") sk = "true"
          else { g = pending; sub(/^gate:/, "", g); addgate(g) }
          pending = ""
        } else if ($0 ~ /t\.Skipf?\("/) {
          # Unclassified skip with a message: record the message so a human can judge it.
          g = $0; sub(/.*t\.Skipf?\("/, "", g); sub(/".*/, "", g); gsub(/\\/, "", g)
          addgate("skip:" g)
        } else if (ttl > 0 && --ttl == 0) {
          pending = ""
        }

        s = $0
        while (match(s, /[A-Za-z_][A-Za-z0-9_]*\(/)) {
          id = substr(s, RSTART, RLENGTH - 1)
          if (!(id in seen)) { seen[id] = 1; calls = calls (calls == "" ? "" : ",") "\"" id "\"" }
          s = substr(s, RSTART + RLENGTH)
        }
      }
      /^}/ { flush() }
      END { flush() }
    ' "$f"
  done
}

# Propagate skip/parallel/gate signals from same-package helpers into the
# Test_ funcs that call them, then keep only top-level tests.
TESTS_JSON="$(extract_funcs | jq -s '
  group_by(.pkg)
  | map(
      (map({key: .name, value: .}) | from_entries) as $m0
      | reduce range(3) as $_ ($m0;
          . as $m
          | with_entries(.value as $f | .value += {
              parallel:  ($f.parallel  or any($f.calls[]; $m[.].parallel  // false)),
              skip_rke2: ($f.skip_rke2 or any($f.calls[]; $m[.].skip_rke2 // false)),
              skip_k3s:  ($f.skip_k3s  or any($f.calls[]; $m[.].skip_k3s  // false)),
              gates:     ([$f.gates[], ($f.calls[] | ($m[.].gates // [])[])] | unique)
            }))
      | [.[] | select(.name | test("^Test_")) | del(.calls)]
    )
  | add // []
  | sort_by(.name)
')" || { echo "error: failed to extract tests from $TESTS_DIR" >&2; exit 2; }

CONFIG_JSON="$(yq -o=json '.' "$CONFIG")" || { echo "error: failed to parse $CONFIG" >&2; exit 2; }

# Paths listed in package-paths/file-paths/full/skip that match no tracked file.
# trigger-* files are created on demand by the 'Create CI Testing PR' workflow.
TRACKED="$(git -C "$ROOT" ls-files)"
DEAD_PATHS="$(
  printf '%s' "$CONFIG_JSON" | jq -r '
      (.scopes | to_entries[] | .key as $s
        | ((.value["package-paths"] // [])[] | {where: "\($s).package-paths", path: .}),
          ((.value["file-paths"] // [])[] | {where: "\($s).file-paths", path: (if type == "object" then .path else . end)})),
      ((.full // [])[] | {where: "full", path: (if type == "object" then .path else . end)}),
      ((.skip // [])[] | {where: "skip", path: (if type == "object" then .path else . end)})
      | "\(.where)\t\(.path)"' \
  | while IFS=$'\t' read -r where path; do
      case "$path" in '^trigger-'*) continue ;; esac
      grep -qE -- "$path" <<<"$TRACKED" || jq -cn --arg w "$where" --arg p "$path" '{where: $w, path: $p}'
    done | jq -s '.'
)"

ANALYSIS="$(jq -n \
  --argjson tests "$TESTS_JSON" \
  --argjson cfg "$CONFIG_JSON" \
  --argjson deadpaths "$DEAD_PATHS" \
  --arg whitelist "$REGEX_WHITELIST" \
  --argjson cap "$MATRIX_CAP" '
  def suite: sub("Test"; "") | gsub("[\\[\\]()|_^.*$?]"; "");
  def skips($t; $d): ($d == "rke2" and $t.skip_rke2) or ($d == "k3s" and $t.skip_k3s);

  # One entry per (row, dist), mirroring the resolver: a row without dist runs on both.
  ([
    ($cfg.scopes // {} | to_entries[] | .key as $s
      | (.value.tests // []) | to_entries[]
      | {kind: "pr", scope: $s, idx: .key, row: .value}),
    ($cfg.explicit // [] | .[] | .name as $s
      | (.tests // []) | to_entries[]
      | {kind: "explicit", scope: $s, idx: .key, row: .value})
  ]
  | map(. as $r | ($r.row.V2PROV_TEST_DIST // ("k3s", "rke2")) as $d
      | {kind, scope, idx, dist: $d, regex: $r.row.V2PROV_TEST_RUN_REGEX,
         cpus: ($r.row.V2PROV_TEST_CPUS // 16), parallelism: ($r.row.V2PROV_TEST_PARALLELISM // 1),
         features: ($r.row.CATTLE_FEATURES // null)})
  ) as $rows

  | ($rows | map(. as $r | . + {
      id: "\(.scope)[\(.idx)]/\(.dist)",
      tests: [$tests[] | select(.name | test($r.regex)) | .name]
    })) as $rows

  | ($tests | map(. as $t | . + {
      rows: [$rows[] | select(.tests | index($t.name)) | {id, kind, scope, dist, skipped: skips($t; .dist)}]
    })) as $placed

  | ($tests | map({key: .name, value: .}) | from_entries) as $byname

  | [
      # Tests no row runs (matched nowhere, or only on dists it skips).
      ($placed[] | select([.rows[] | select(.skipped | not)] | length == 0)
        | if (.gates | length) > 0
          then "ORPHAN-GATED \(.name) (\(.file):\(.line)) gated by \(.gates | join(","))"
          else "ORPHAN \(.name) (\(.file):\(.line))" end),

      # Runs on a PR for a dist, but nightly never runs it on that dist.
      ($placed[] as $t
        | ([$t.rows[] | select(.kind == "pr" and (.skipped | not)) | .dist] | unique) as $pr
        | ([$t.rows[] | select(.scope == "nightly" and (.skipped | not)) | .dist] | unique) as $nightly
        | ($pr - $nightly)[]
        | "NOT-IN-NIGHTLY \($t.name) on \(.)"),

      # Same test picked up twice in one run.
      ($placed[] as $t
        | ($t.rows | group_by([.kind, (if .kind == "pr" then "" else .scope end), .dist])[]
          | select(length > 1)
          | "DUPLICATE \($t.name) on \(.[0].dist) via \(map(.id) | join(" "))")),

      # Rows that match nothing, or only tests that skip on that dist.
      ($rows[] | select(.tests | length == 0) | "DEAD-ROW \(.id) \(.regex)"),
      ($rows[] | . as $r | select((.tests | length) > 0 and all(.tests[]; skips($byname[.]; $r.dist)))
        | "WASTED-DIST \(.id) every matched test skips on \(.dist)"),

      # Rejected at runtime by scripts/provisioning-tests.
      ($rows | unique_by([.scope, .idx])[] | select(.regex | test($whitelist) | not)
        | "INVALID-REGEX \(.scope)[\(.idx)] \(.regex)"),

      # parallelism > 1 does nothing unless a matched (non-skipped) test calls t.Parallel().
      ($rows[] | . as $r | select(.parallelism > 1)
        | select([.tests[] | $byname[.] | select((skips(.; $r.dist) | not) and .parallel)] | length == 0)
        | "NOOP-PARALLELISM \(.id) parallelism=\(.parallelism) but no matched test calls t.Parallel()"),

      # Two rows in one run that upload the same artifact name.
      (($rows | map(. + {run: (if .kind == "pr" then "pr" else .scope end), suite: (.regex | suite)}))
        | group_by([.run, .dist, .suite])[] | select(length > 1)
        | "ARTIFACT-COLLISION \(map(.id) | join(" ")) share suite \(.[0].suite)"),

      # Matrix fan-out over the sanity cap (all PR scopes together is what a full trigger produces).
      (($rows | map(select(.kind == "pr")) | length) as $n
        | select($n > $cap) | "MATRIX-CAP pr(full) has \($n) rows, cap is \($cap)"),
      ($rows | map(select(.kind == "explicit")) | group_by(.scope)[]
        | select(length > $cap) | "MATRIX-CAP \(.[0].scope) has \(length) rows, cap is \($cap)"),

      ($deadpaths[] | "DEAD-PATH \(.where) \(.path) matches no tracked file")
    ] as $problems

  | {
      tests: $placed,
      rows: $rows,
      problems: ($problems | unique),
      capacity: (
        [{scope: "pr(full)", rows: ($rows | map(select(.kind == "pr")) | length)}]
        + ($rows | map(select(.kind == "explicit")) | group_by(.scope) | map({scope: .[0].scope, rows: length}))
        | map(. + {cap: $cap}))
    }
')" || { echo "error: analysis failed" >&2; exit 2; }

print_problems() {
  printf '%s' "$ANALYSIS" | jq -r '
    if (.problems | length) == 0 then "Problems: none"
    else "Problems (\(.problems | length)):", (.problems[] | "  \(.)") end'
}

print_capacity() {
  printf '%s' "$ANALYSIS" | jq -r '"Matrix capacity:", (.capacity[] | "  \(.scope): \(.rows)/\(.cap)")'
}

case "$MODE" in
  json)
    printf '%s\n' "$ANALYSIS"
    ;;
  problems)
    printf '%s' "$ANALYSIS" | jq -r '.problems[]'
    ;;
  test)
    printf '%s' "$ANALYSIS" | jq -e --arg n "$TEST_NAME" '.tests[] | select(.name == $n)' >/dev/null \
      || { echo "error: no top-level test named $TEST_NAME under tests/v2prov/tests" >&2; exit 2; }
    printf '%s' "$ANALYSIS" | jq -r --arg n "$TEST_NAME" '
      .tests[] | select(.name == $n)
      | "\(.name)  \(.file):\(.line)",
        "  t.Parallel: \(.parallel)   skips: \([if .skip_k3s then "k3s" else empty end, if .skip_rke2 then "rke2" else empty end] | join(",") | if . == "" then "-" else . end)   gates: \(.gates | join(",") | if . == "" then "-" else . end)",
        (if (.rows | length) == 0 then "  matched by no row"
         else (.rows[] | "  \(.kind)\t\(.id)\(if .skipped then "  (skips on this dist)" else "" end)") end)'
    echo
    print_problems
    ;;
  table)
    printf '%s' "$ANALYSIS" | jq -r '
      ["TEST", "PR", "NIGHTLY", "KDM", "PAR", "SKIPS", "GATES"],
      (.tests[] | . as $t
        | def ids(f): [.rows[] | select(f and (.skipped | not)) | .id] | join(" ") | if . == "" then "-" else . end;
        [ .name,
          ids(.kind == "pr"),
          ids(.scope == "nightly"),
          ids(.scope == "kdm"),
          (if .parallel then "y" else "-" end),
          ([if .skip_k3s then "k3s" else empty end, if .skip_rke2 then "rke2" else empty end] | join(",") | if . == "" then "-" else . end),
          (.gates | join(",") | if . == "" then "-" else . end) ])
      | @tsv' | column -t -s $'\t'
    echo
    print_capacity
    echo
    print_problems
    ;;
esac

[ "$(printf '%s' "$ANALYSIS" | jq '.problems | length')" -eq 0 ]
