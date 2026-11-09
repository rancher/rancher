---
name: v2prov-ci-scopes
description: Place new or changed v2prov provisioning tests into the CI shards in .github/scripts/provisioning-test-scopes.yaml, keeping shards balanced and path triggers accurate. Use when someone adds, renames, moves or deletes a test under tests/v2prov/tests, asks which scope or shard a test runs in, wants to rebalance provisioning CI, or edits provisioning-test-scopes.yaml.
---

# v2prov CI scopes

This skill helps a test author get their test into the right CI shard. It does five things:

- Reads the test to work out its cost and behavior.
- Interviews the author about anything the code can't answer.
- Proposes edits to `.github/scripts/provisioning-test-scopes.yaml`.
- Checks those edits mechanically before writing them.
- Reports existing config drift separately.

Read [reference.md](reference.md) before deciding placement. It has the scope meanings, naming tiers, cost model, hard constraints and balancing rules.

Scripts live in `scripts/` next to this file. Run them from anywhere inside the repo:

- `scripts/check-coverage.sh` maps every top-level test to the rows that run it and lists problems. Use `--test <name>` for one test, `--problems` for a diffable problem list, and `--json` for raw data. `SCOPES_CONFIG=<file>` points it at a proposed copy.
- `scripts/fetch-timings.sh --branch <branch>` gets median per-test and per-shard durations from recent nightly runs on that branch only. Exit 3 means there's no nightly data for that branch.

## Workflow

### 1. Find the tests and capture a baseline

- Work out which tests are in play. Use the names the author gives, or find new and changed top-level `func Test_` declarations in `git diff <base>...HEAD -- tests/v2prov/tests`.
- Determine the target branch: the branch the work will merge into (the upstream tracking branch, or the PR's base). Confirm it with the author, and don't guess silently. Every timing number below is only valid for that branch.
- Save the baseline: `scripts/check-coverage.sh --problems > /tmp/v2prov-baseline.txt`. Problems already in this list are pre-existing drift, not caused by the author's change.

For each test, read the code and note:

- **Helper and node count:** `systemdnode` (custom), `MachinePools[].Quantity` (MP), or `setUpImportedCluster` pools (imported). Also note which roles each node has.
- **What it does:** create, scale, drain, delete, rotate certs, snapshot or restore, and so on. Note which Rancher objects it creates or asserts on (plan secrets, etcd snapshot CRs, machine objects, fleet clusters).
- **Dist skips, env or feature gates, and `t.Parallel()`.** Compare what you read with `scripts/check-coverage.sh --test <name>`. The script's detection is heuristic, so trust the code when they disagree.
- **Where it lands today**, according to the same command.

### 2. Interview the author

Use AskUserQuestion. Ask only what the code didn't settle, and pre-fill a recommended option from what you read. Batch related questions (up to 4 per call). Cover:

1. **Purpose**, when the name and directory don't make it obvious. Options: first-time provisioning flow, day-2 operation, imported cluster behavior, fleet or agent behavior, or a feature-flagged path.
2. **Exercised code.** Ask which Rancher packages or controllers this test is meant to protect.
   - Propose candidates from what the test touches. For example, etcd snapshot CRs point to `pkg/controllers/capr/etcdmgmt/`, drain annotations to `pkg/controllers/capr/machinedrain/`, and imported day-2 ops to `pkg/controllers/capr/unmanaged/` or `pkg/operations/`.
   - Let the author add or remove candidates.
   - This list drives the `package-paths` edits.
3. **Trigger scenarios.** What kind of change should make this test run? Examples: "any planner change", "changes to the snapshot restore path", "fleet version bumps".
   - This decides PR tier versus nightly-only.
   - It also shows whether a `file-paths` substring rule fits better than a whole package. Dependency bumps in `go.mod` and version pins in `build.yaml` or `pkg/settings/setting.go` are examples.
4. **Distros**, if the test doesn't already restrict itself. Should it run on k3s, rke2, or both?
5. **kdm.** Should KDM validation run it? Recommend no, unless the behavior depends on the Kubernetes version.
6. **Runtime**, only if `fetch-timings.sh` has no data for this test. Ask how long it took locally, on which dist, and how many nodes were up at once.

If an answer conflicts with the code, raise it rather than picking one. An example is "should run on k3s" for a test that skips on k3s.

### 3. Decide placement

Work through these in order:

1. **Scope.** Pick it from the purpose and the test directory (see the scope table in reference.md).
2. **Tier.** PR scope plus nightly, or nightly only. Apply the "Should a test run on PRs?" rules.
3. **Name.** Check that the test name lands in the intended row.
   - If it doesn't, propose a rename to the right prefix and Set letter before touching any regex.
   - Renaming is almost always better than widening a regex.
4. **Row.**
   - Run `scripts/fetch-timings.sh --branch <target>`.
   - On exit 3, tell the author there's no nightly data for that branch and use the cost model in reference.md. Never borrow numbers from another branch, and never use PR runs.
   - Pick the lightest compatible row. Only split if adding the test pushes the shard past the balancing target. Follow the split rules in reference.md, and keep rows disjoint.
5. **Row options.** Set `V2PROV_TEST_CPUS` and `V2PROV_TEST_PARALLELISM` from node count and `t.Parallel()` usage, and `CATTLE_FEATURES` from any feature gate.
6. **Nightly.** Make sure a nightly row covers the test on every dist it runs on in PRs, and on any extra dists it should run on nightly.
   - Watch the 25-row cap. Nightly is close to it, so prefer widening an existing nightly row's alternation over adding a row.
7. **kdm.** Add the test only if the author agreed.
8. **Trigger paths.** For each package the author confirmed:
   - If no scope lists it, add it to the chosen scope's `package-paths`, in the existing style (anchored `'^pkg/.../'`, keeping the file's grouping and alignment).
   - If another scope already lists it, add it here too with `# also <scope>`, and update the comment on the other entry.
   - If it's under `skip`, the author is closing a coverage gap. Move it out of `skip` and say so.
   - If it already matches `full`, no path edit is needed.
   - Also make sure the test's own directory is in the scope's `package-paths`.

### 4. Propose and verify before writing

- Copy the config to a temp file, apply the proposed edits there, then run:
  ```
  SCOPES_CONFIG=/tmp/scopes.yaml scripts/check-coverage.sh --test <name>
  SCOPES_CONFIG=/tmp/scopes.yaml scripts/check-coverage.sh --problems > /tmp/v2prov-proposed.txt
  diff /tmp/v2prov-baseline.txt /tmp/v2prov-proposed.txt
  ```
  The diff must not add problems. The only exception is one the author accepts knowingly. `ORPHAN-GATED` for a deliberately local-only test is an example.
- For each new or changed trigger path, pick one real file under it and confirm the scope fires:
  ```
  SCOPES_CONFIG=/tmp/scopes.yaml CHANGED=<file> .github/scripts/resolve-provisioning-matrix.sh
  ```
  The stderr should name the scope, and the JSON should include the row that runs the test.
- Show the author:
  - The YAML diff.
  - The rows the test will run in (PR, nightly, kdm), per dist.
  - The change in matrix row count for the PR union and for nightly.
  - The estimated shard time before and after, saying whether it came from nightly data (with the run IDs) or from the cost model.

### 5. Apply

Write the edits to the real config only after the author confirms. Then run `scripts/check-coverage.sh --test <name>` and the `--problems` diff against the baseline once more.

### 6. Report pre-existing drift

List the problems from the baseline that aren't related to this change; reference.md describes the known ones. Offer a fix for each, and apply only the fixes the author picks. Keep them as separate edits so they can go in their own commit.

## Rules

- Never widen a regex to `.*` or a bare prefix just to catch one test.
- Never let two rows in one scope match the same test on the same dist.
- Never edit the workflows, the resolver or `scripts/provisioning-tests` as part of placing a test. If a constraint there is the real blocker, say so.
- Keep the YAML's existing alignment, quoting and comment style.
- Row keys pass straight through to the matrix. When splitting or copying a row, carry over every key it has, including ones this repo doesn't read (such as `V2PROV_TEST_RUN_ON_RELEASE`).
- When removing or renaming a test, check that no row becomes dead (`DEAD-ROW`) and that no enumerated alternation still lists the old name.
